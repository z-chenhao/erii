package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/z-chenhao/erii/internal/llm"
	"github.com/z-chenhao/erii/internal/llm/memory"
)

func modelInput() llm.ProfileInput {
	return llm.ProfileInput{Name: "Local candidate", ProviderKind: "openai_compatible", Endpoint: "http://127.0.0.1:8000/v1", WireAPI: "openai_completions", ModelID: "fixture-model", ContextWindow: 8192, MaxOutputTokens: 1024,
		Compatibility: llm.Compatibility{ToolResultFormat: "openai"}, Capabilities: llm.Capabilities{TextInput: true}}
}

func openModels(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func countRows(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestModelMetadataRestartCASAndAtomicEvents(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.Diagnostics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Models().AddProfile(ctx, modelInput())
	if err != nil {
		t.Fatal(err)
	}
	id, err := uuid.Parse(p.ProfileID)
	if err != nil || id.Version() != 7 {
		t.Fatal("profile ID is not UUIDv7")
	}
	if p.Status != "unverified" || p.VerifiedAt != nil || p.Version != 1 || p.CreatedAt.Location() != time.UTC || p.CreatedAt.Nanosecond()%1000 != 0 {
		t.Fatalf("invalid metadata: %+v", p)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	again, err := s.Models().Profile(ctx, p.ProfileID)
	if err != nil || !reflect.DeepEqual(p, again) {
		t.Fatalf("restart changed metadata: %+v %v", again, err)
	}
	input := modelInput()
	input.Name = "Changed"
	updated, err := s.Models().UpdateProfile(ctx, p.ProfileID, 1, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 || updated.Name != "Changed" || updated.CreatedAt != p.CreatedAt {
		t.Fatal("CAS did not update metadata")
	}
	events := countRows(t, s, "event_outbox")
	if _, err := s.Models().UpdateProfile(ctx, p.ProfileID, 1, input); !errors.Is(err, llm.ErrConflict) {
		t.Fatalf("stale CAS: %v", err)
	}
	if events != countRows(t, s, "event_outbox") {
		t.Fatal("stale CAS wrote event")
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_model_event BEFORE INSERT ON event_outbox BEGIN SELECT RAISE(ABORT,'fault'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Models().UpdateProfile(ctx, p.ProfileID, 2, input); !errors.Is(err, llm.ErrStorage) {
		t.Fatalf("expected atomic failure: %v", err)
	}
	if _, err := s.Models().AddProfile(ctx, input); !errors.Is(err, llm.ErrStorage) {
		t.Fatalf("expected add rollback: %v", err)
	}
	stored, err := s.Models().Profile(ctx, p.ProfileID)
	if err != nil || !reflect.DeepEqual(updated, stored) || countRows(t, s, "model_profile") != 1 {
		t.Fatal("failed event changed profiles")
	}
	after, err := s.Diagnostics(ctx)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("metadata enabled ingress or changed agent configuration")
	}
}

func TestConcurrentProfileCASAcrossConnections(t *testing.T) {
	ctx := context.Background()
	s := openModels(t)
	other, err := Open(ctx, filepath.Dir(s.path))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	p, err := s.Models().AddProfile(ctx, modelInput())
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, store := range []*Store{s, other} {
		go func() {
			<-start
			_, err := store.Models().UpdateProfile(ctx, p.ProfileID, 1, modelInput())
			results <- err
		}()
	}
	close(start)
	wins, conflicts := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			wins++
		} else if errors.Is(err, llm.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 1 || countRows(t, s, "event_outbox") != 3 {
		t.Fatal("CAS allowed lost update or duplicate event")
	}
}

func TestModelConstraintsAndDanglingAuthentication(t *testing.T) {
	ctx := context.Background()
	s := openModels(t)
	input := modelInput()
	input.AuthConnectionID = uuid.Must(uuid.NewV7()).String()
	if _, err := s.Models().AddProfile(ctx, input); !errors.Is(err, llm.ErrStorage) {
		t.Fatalf("dangling auth accepted: %v", err)
	}
	a, err := llm.RegisterAuth(ctx, s.Models(), memory.New(), llm.AuthInput{Provider: "openai", AuthKind: "chatgpt_oauth"}, []byte("synthetic-fixture"))
	if err != nil {
		t.Fatal(err)
	}
	input.AuthConnectionID = a.ConnectionID
	if _, err := s.Models().AddProfile(ctx, input); err == nil {
		t.Fatal("OAuth accepted for API key path")
	}
	input.ProviderKind, input.Endpoint, input.WireAPI = "openai_account", "", "openai_responses"
	p, err := s.Models().AddProfile(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	queries := []string{
		`UPDATE model_profile SET status='invalid-state',version=version+1`,
		`UPDATE model_profile SET status='ready',version=version+1`,
		`UPDATE model_profile SET version=0`,
		`UPDATE model_profile SET name='no-cas'`,
		`UPDATE model_profile SET context_window=0,version=version+1`,
		`UPDATE model_profile SET max_output_tokens=context_window+1,version=version+1`,
		`UPDATE model_profile SET generation_defaults_json='[]',version=version+1`,
		`UPDATE model_profile SET capabilities_json='bad',version=version+1`,
		`UPDATE model_profile SET auth_connection_id='missing',version=version+1`,
		`UPDATE model_profile SET profile_id='new',version=version+1`,
		`UPDATE model_profile SET provider_kind='fake',version=version+1`,
		`UPDATE model_profile SET wire_api='fake',version=version+1`,
		`UPDATE model_profile SET updated_at=0,version=version+1`,
		`UPDATE auth_connection SET status='bad',version=version+1`,
		`UPDATE auth_connection SET credential_ref='different',version=version+1`,
		`UPDATE auth_connection SET auth_kind='api_key',version=version+1`,
		`DELETE FROM auth_connection`,
		`INSERT OR REPLACE INTO model_profile SELECT * FROM model_profile`,
		`INSERT OR REPLACE INTO auth_connection SELECT * FROM auth_connection`,
	}
	for _, query := range queries {
		if _, err := s.db.Exec(query); err == nil {
			t.Errorf("accepted invalid state: %s", query)
		}
	}
	again, err := s.Models().Profile(ctx, p.ProfileID)
	if err != nil || !reflect.DeepEqual(p, again) {
		t.Fatal("constraint rejection changed profile")
	}
}

// markReady is a storage fixture, not a Probe or a production activation path.
func markReady(t *testing.T, s *Store, p llm.Profile) llm.Profile {
	t.Helper()
	if _, err := s.db.Exec(`UPDATE model_profile SET status='ready',verified_at=?,version=version+1 WHERE profile_id=?`, time.Now().UTC().UnixMicro(), p.ProfileID); err != nil {
		t.Fatal(err)
	}
	p, err := s.Models().Profile(context.Background(), p.ProfileID)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSnapshotImmutabilityFrozenValuesAndRollback(t *testing.T) {
	ctx := context.Background()
	s := openModels(t)
	secrets := memory.New()
	a, err := llm.RegisterAuth(ctx, s.Models(), secrets, llm.AuthInput{Provider: "local", AuthKind: "api_key"}, []byte("synthetic-fixture"))
	if err != nil {
		t.Fatal(err)
	}
	input := modelInput()
	input.AuthConnectionID = a.ConnectionID
	p, err := s.Models().AddProfile(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Transaction(ctx, func(tx *sql.Tx) error {
		_, err := s.Models().FreezeProfile(ctx, tx, p.ProfileID, p.Version)
		return err
	})
	if !errors.Is(err, llm.ErrUnverified) || countRows(t, s, "model_snapshot") != 0 {
		t.Fatal("unverified profile frozen")
	}
	p = markReady(t, s, p)
	var frozen llm.Snapshot
	err = s.Transaction(ctx, func(tx *sql.Tx) error {
		var err error
		frozen, err = s.Models().FreezeProfile(ctx, tx, p.ProfileID, p.Version)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if frozen.ProfileVersion != p.Version || len(frozen.EndpointHash) != 64 || frozen.AuthConnectionID != a.ConnectionID {
		t.Fatal("snapshot missing frozen metadata")
	}
	id, err := uuid.Parse(frozen.SnapshotID)
	if err != nil || id.Version() != 7 {
		t.Fatal("snapshot identity not UUIDv7")
	}
	encoded, err := json.Marshal(frozen)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "credential") || bytes.Contains(encoded, []byte(a.CredentialRef)) || bytes.Contains(encoded, []byte(p.Endpoint)) {
		t.Fatal("snapshot contains auth locator or endpoint")
	}
	for _, query := range []string{`UPDATE model_snapshot SET model_id='mutated'`, `DELETE FROM model_snapshot`, `INSERT OR REPLACE INTO model_snapshot SELECT * FROM model_snapshot`, `DELETE FROM model_profile`} {
		if _, err := s.db.Exec(query); err == nil {
			t.Errorf("snapshot mutation accepted: %s", query)
		}
	}
	input.ModelID, input.Endpoint = "next-model", "http://localhost:9000/v1"
	updated, err := s.Models().UpdateProfile(ctx, p.ProfileID, p.Version, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "unverified" || updated.VerifiedAt != nil {
		t.Fatal("metadata change retained verification")
	}
	refreshed := time.Now().UTC().Truncate(time.Microsecond)
	if err := s.Models().UpdateAuthState(ctx, a.ConnectionID, 1, "connected", nil, &refreshed); err != nil {
		t.Fatal(err)
	}
	if err := s.Models().UpdateAuthState(ctx, a.ConnectionID, 1, "connected", nil, nil); !errors.Is(err, llm.ErrConflict) {
		t.Fatal("stale auth CAS accepted")
	}
	stored, err := s.Models().Snapshot(ctx, frozen.SnapshotID)
	if err != nil || !reflect.DeepEqual(stored, frozen) {
		t.Fatal("profile/auth update changed snapshot")
	}
	updated = markReady(t, s, updated)
	events := countRows(t, s, "event_outbox")
	rollback := errors.New("future Run write failed")
	err = s.Transaction(ctx, func(tx *sql.Tx) error {
		_, err := s.Models().FreezeProfile(ctx, tx, updated.ProfileID, updated.Version)
		if err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) || countRows(t, s, "model_snapshot") != 1 || countRows(t, s, "event_outbox") != events {
		t.Fatal("snapshot committed without future Run transaction")
	}
	if err := s.Models().UpdateAuthState(ctx, a.ConnectionID, 2, "reauth_required", nil, nil); err != nil {
		t.Fatal(err)
	}
	err = s.Transaction(ctx, func(tx *sql.Tx) error {
		_, err := s.Models().FreezeProfile(ctx, tx, updated.ProfileID, updated.Version)
		return err
	})
	if !errors.Is(err, llm.ErrUnverified) {
		t.Fatal("frozen with unavailable authentication")
	}
	err = s.Transaction(ctx, func(tx *sql.Tx) error { _, err := s.Models().FreezeProfile(ctx, tx, updated.ProfileID, 1); return err })
	if !errors.Is(err, llm.ErrConflict) {
		t.Fatal("snapshot accepted stale profile version")
	}
	dir := filepath.Dir(s.path)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restored, err := reopened.Models().Snapshot(ctx, frozen.SnapshotID)
	if err != nil || !reflect.DeepEqual(restored, frozen) {
		t.Fatal("restart changed immutable snapshot")
	}
	restoredAuth, err := reopened.Models().Auth(ctx, a.ConnectionID)
	if err != nil || restoredAuth.Version != 3 || restoredAuth.Status != "reauth_required" || restoredAuth.CredentialRef != a.CredentialRef {
		t.Fatal("restart changed auth metadata")
	}
}

type failingSecrets struct {
	*memory.SecretStore
	put, del bool
	lastRef  string
}

func (f *failingSecrets) Put(ctx context.Context, ref string, value []byte) error {
	f.lastRef = ref
	if f.put {
		return errors.New("SYNTHETIC_ERROR_VALUE")
	}
	return f.SecretStore.Put(ctx, ref, value)
}
func (f *failingSecrets) Delete(ctx context.Context, ref string) error {
	if f.del {
		return errors.New("SYNTHETIC_ERROR_VALUE")
	}
	return f.SecretStore.Delete(ctx, ref)
}

func TestAuthSecretFailureCleanupAndRedaction(t *testing.T) {
	ctx := context.Background()
	s := openModels(t)
	fake := &failingSecrets{SecretStore: memory.New(), put: true}
	input := llm.AuthInput{Provider: "local", AuthKind: "api_key", AccountLabel: "fixture-account"}
	if _, err := llm.RegisterAuth(ctx, s.Models(), fake, input, []byte("SYNTHETIC_CREDENTIAL_VALUE")); !errors.Is(err, llm.ErrSecretStore) || strings.Contains(err.Error(), "SYNTHETIC") {
		t.Fatalf("unsafe secret error: %v", err)
	}
	if countRows(t, s, "auth_connection") != 0 || countRows(t, s, "event_outbox") != 1 {
		t.Fatal("Put failure wrote domain state")
	}
	fake.put = false
	if _, err := s.db.Exec(`CREATE TRIGGER fail_auth_event BEFORE INSERT ON event_outbox BEGIN SELECT RAISE(ABORT,'SYNTHETIC_ERROR_VALUE'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := llm.RegisterAuth(ctx, s.Models(), fake, input, []byte("SYNTHETIC_CREDENTIAL_VALUE")); !errors.Is(err, llm.ErrStorage) || strings.Contains(err.Error(), "SYNTHETIC") {
		t.Fatalf("unsafe database error: %v", err)
	}
	if _, err := fake.Get(ctx, fake.lastRef); !errors.Is(err, memory.ErrMissing) {
		t.Fatal("failed transaction left credential behind")
	}
	if countRows(t, s, "auth_connection") != 0 {
		t.Fatal("failed transaction left auth reference")
	}
	fake.del = true
	if _, err := llm.RegisterAuth(ctx, s.Models(), fake, input, []byte("SYNTHETIC_CREDENTIAL_VALUE")); !errors.Is(err, llm.ErrSecretCleanup) || strings.Contains(err.Error(), "SYNTHETIC") {
		t.Fatalf("cleanup failure hidden or leaked: %v", err)
	}
	fake.del = false
	if err := fake.Delete(ctx, fake.lastRef); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DROP TRIGGER fail_auth_event`); err != nil {
		t.Fatal(err)
	}
	a, err := llm.RegisterAuth(ctx, s.Models(), fake, input, []byte("SYNTHETIC_CREDENTIAL_VALUE"))
	if err != nil {
		t.Fatal(err)
	}
	stored, err := s.Models().Auth(ctx, a.ConnectionID)
	if err != nil || !reflect.DeepEqual(stored, a) {
		t.Fatalf("auth readback: %v", err)
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(a.CredentialRef)) || bytes.Contains(encoded, []byte("credential_ref")) {
		t.Fatal("credential ref serialized")
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		data, err := os.ReadFile(s.path + suffix)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte("SYNTHETIC_CREDENTIAL_VALUE")) {
			t.Fatal("credential material reached database")
		}
	}
}

func TestAuthCommitFailureCleansCredentialAndRollsBackEvent(t *testing.T) {
	ctx := context.Background()
	s := openModels(t)
	// SQLite accepts the writes, then rejects COMMIT because this deferred
	// relationship is unsatisfied. Exercise a real commit error, not just DML.
	if _, err := s.db.Exec(`CREATE TABLE commit_guard (connection_id TEXT REFERENCES auth_connection(connection_id) DEFERRABLE INITIALLY DEFERRED);
CREATE TRIGGER fail_auth_commit AFTER INSERT ON auth_connection BEGIN INSERT INTO commit_guard VALUES('missing'); END`); err != nil {
		t.Fatal(err)
	}
	fake := &failingSecrets{SecretStore: memory.New()}
	_, err := llm.RegisterAuth(ctx, s.Models(), fake, llm.AuthInput{Provider: "local", AuthKind: "api_key"}, []byte("fixture"))
	if !errors.Is(err, llm.ErrStorage) {
		t.Fatalf("expected commit failure: %v", err)
	}
	if countRows(t, s, "auth_connection") != 0 || countRows(t, s, "commit_guard") != 0 || countRows(t, s, "event_outbox") != 1 {
		t.Fatal("failed commit persisted metadata or event")
	}
	if _, err := fake.Get(ctx, fake.lastRef); !errors.Is(err, memory.ErrMissing) {
		t.Fatal("commit failure retained credential")
	}
}

func TestRealPriorSchemasUpgradeAndV3Rollback(t *testing.T) {
	for _, prior := range []int{1, 2} {
		t.Run(string(rune('0'+prior)), func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			db, err := sql.Open("sqlite", filepath.Join(dir, "state.db"))
			if err != nil {
				t.Fatal(err)
			}
			if err := migrate(ctx, db, migrations[:prior]); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO agent_config VALUES(1,'preserved-agent',NULL,'setup_required',7,100,200)`); err != nil {
				t.Fatal(err)
			}
			if prior == 2 {
				if _, err := db.Exec(`INSERT INTO event_outbox VALUES('outbox','event','subject','type','{}',101,102,3); INSERT INTO event_inbox VALUES('consumer','event',103)`); err != nil {
					t.Fatal(err)
				}
			}
			broken := append(append([]string{}, migrations[:2]...), modelsSQL+"; INVALID SQL")
			if err := migrate(ctx, db, broken); err == nil {
				t.Fatal("expected v3 migration failure")
			}
			var version, tables int
			if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name IN ('model_profile','model_snapshot','auth_connection')`).Scan(&tables); err != nil {
				t.Fatal(err)
			}
			if version != prior || tables != 0 {
				t.Fatal("failed migration changed prior schema")
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				s, err := Open(ctx, dir)
				if err != nil {
					t.Fatal(err)
				}
				d, err := s.Diagnostics(ctx)
				if err != nil || d.AgentID != "preserved-agent" || d.ConfigVersion != 7 || d.SchemaVersion != 17 || d.Readiness != "setup_required" {
					t.Fatal("migration changed agent")
				}
				var created, updated int64
				if err := s.db.QueryRow(`SELECT created_at,updated_at FROM agent_config`).Scan(&created, &updated); err != nil {
					t.Fatal(err)
				}
				if created != 100 || updated != 200 {
					t.Fatal("migration changed timestamps")
				}
				if prior == 2 {
					var row string
					if err := s.db.QueryRow(`SELECT json_array(outbox_id,event_id,subject,type,payload_json,occurred_at,published_at,attempt_count) FROM event_outbox`).Scan(&row); err != nil {
						t.Fatal(err)
					}
					if row != `["outbox","event","subject","type","{}",101,102,3]` || countRows(t, s, "event_inbox") != 1 {
						t.Fatal("migration changed event history")
					}
				} else if countRows(t, s, "event_outbox") != 0 {
					t.Fatal("migration fabricated historical events")
				}
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestSecretFakeOwnsCopiesAndConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	fake := memory.New()
	value := []byte("fixture")
	if err := fake.Put(ctx, "ref", value); err != nil {
		t.Fatal(err)
	}
	value[0] = 'X'
	got, err := fake.Get(ctx, "ref")
	if err != nil || string(got) != "fixture" {
		t.Fatal("Put aliased input")
	}
	got[0] = 'X'
	got, err = fake.Get(ctx, "ref")
	if err != nil || string(got) != "fixture" {
		t.Fatal("Get aliased stored credential")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 20 {
				if err := fake.Put(ctx, "ref", []byte("fixture")); err != nil {
					t.Error(err)
				}
				if _, err := fake.Get(ctx, "ref"); err != nil {
					t.Error(err)
				}
			}
		})
	}
	wg.Wait()
	if err := fake.Delete(ctx, "ref"); err != nil {
		t.Fatal(err)
	}
	if err := fake.Delete(ctx, "ref"); err != nil {
		t.Fatal(err)
	}
	if _, err := fake.Get(ctx, "ref"); !errors.Is(err, memory.ErrMissing) {
		t.Fatal("Delete retained credential")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := fake.Put(canceled, "ref", value); err == nil {
		t.Fatal("canceled Put succeeded")
	}
	if _, err := fake.Get(canceled, "ref"); err == nil {
		t.Fatal("canceled Get succeeded")
	}
	if err := fake.Delete(canceled, "ref"); err == nil {
		t.Fatal("canceled Delete succeeded")
	}
}
