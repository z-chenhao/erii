package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/z-chenhao/erii/internal/llm"
	"github.com/z-chenhao/erii/internal/llm/memory"
)

type probeFixture struct {
	result llm.ProbeResult
	err    error
}

func (p probeFixture) Run(context.Context, llm.Profile) (llm.ProbeResult, error) {
	return p.result, p.err
}
func passedProbe() llm.ProbeResult {
	return llm.ProbeResult{Streaming: true, ToolCalls: true, Continuation: true, Cancellation: true, Clipping: true}
}
func control(s *Store) llm.Control {
	return llm.Control{Store: s.Models(), Probe: probeFixture{result: passedProbe()}, BridgeIdentity: "test:success"}
}

func TestAtomicActivationRollbackAndFencing(t *testing.T) {
	ctx := context.Background()
	s := openModels(t)
	c := control(s)
	p, err := s.Models().AddProfile(ctx, modelInput())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Select(ctx, p.ProfileID, 1, 1); !errors.Is(err, llm.ErrUnverified) {
		t.Fatalf("unprobed activation: %v", err)
	}
	if _, err := c.Test(ctx, p.ProfileID); err != nil {
		t.Fatal(err)
	}
	before, err := s.Diagnostics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	count := countRows(t, s, "event_outbox")
	if _, err := s.db.Exec(`CREATE TRIGGER fail_selection BEFORE INSERT ON event_outbox WHEN NEW.type='ai.erii.agent.model_selected.v1' BEGIN SELECT RAISE(ABORT,'fault'); END`); err != nil {
		t.Fatal(err)
	}
	if err := c.Select(ctx, p.ProfileID, 1, 1); !errors.Is(err, llm.ErrStorage) {
		t.Fatalf("missing failure: %v", err)
	}
	stored, err := s.Models().Profile(ctx, p.ProfileID)
	if err != nil || !reflect.DeepEqual(p, stored) {
		t.Fatal("failed commit changed profile", err)
	}
	after, err := s.Diagnostics(ctx)
	if err != nil || !reflect.DeepEqual(before, after) || countRows(t, s, "event_outbox") != count {
		t.Fatal("failed commit changed config/events", err)
	}
	if _, err := s.db.Exec(`DROP TRIGGER fail_selection`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TABLE fail_activation_commit(id TEXT REFERENCES model_profile(profile_id) DEFERRABLE INITIALLY DEFERRED);
CREATE TRIGGER defer_activation_failure AFTER UPDATE OF active_model_profile_id ON agent_config BEGIN INSERT INTO fail_activation_commit VALUES('missing'); END`); err != nil {
		t.Fatal(err)
	}
	if err := c.Select(ctx, p.ProfileID, 1, 1); !errors.Is(err, llm.ErrStorage) {
		t.Fatalf("COMMIT failure not surfaced: %v", err)
	}
	stored, err = s.Models().Profile(ctx, p.ProfileID)
	if err != nil || !reflect.DeepEqual(p, stored) || countRows(t, s, "event_outbox") != count {
		t.Fatal("COMMIT failure did not roll back profile/events", err)
	}
	after, err = s.Diagnostics(ctx)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("COMMIT failure did not roll back config", err)
	}
	if _, err := s.db.Exec(`DROP TRIGGER defer_activation_failure; DROP TABLE fail_activation_commit`); err != nil {
		t.Fatal(err)
	}
	if err := c.Select(ctx, p.ProfileID, 1, 999); !errors.Is(err, llm.ErrConflict) {
		t.Fatalf("stale config: %v", err)
	}
	if err := c.Select(ctx, p.ProfileID, 1, 1); err != nil {
		t.Fatal(err)
	}
	stored, err = s.Models().Profile(ctx, p.ProfileID)
	if err != nil || stored.Status != "ready" || stored.Version != 2 || stored.VerifiedAt == nil || !c.Available(ctx, p.ProfileID) {
		t.Fatal("activation not verified", stored, err)
	}
	after, err = s.Diagnostics(ctx)
	if err != nil || after.ConfigVersion != 2 || after.ActiveModelProfileID == nil || *after.ActiveModelProfileID != p.ProfileID {
		t.Fatal(after, err)
	}
	if err := c.Select(ctx, p.ProfileID, 1, 1); !errors.Is(err, llm.ErrConflict) {
		t.Fatal("old selection accepted", err)
	}
	c.BridgeIdentity = "different"
	if c.Available(ctx, p.ProfileID) {
		t.Fatal("Fake provenance bypass")
	}
	c = control(s)
	if _, err := s.Models().UpdateProfile(ctx, p.ProfileID, 2, modelInput()); err != nil {
		t.Fatal(err)
	}
	if c.Available(ctx, p.ProfileID) {
		t.Fatal("active edit kept verification")
	}
	d, err := s.Diagnostics(ctx)
	if err != nil || d.Readiness != "degraded" || d.AgentIngressEnabled {
		t.Fatal("active edit did not close ingress", d, err)
	}
}

func TestActivationConcurrentConfigCAS(t *testing.T) {
	ctx := context.Background()
	s := openModels(t)
	other, err := Open(ctx, filepath.Dir(s.path))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	a, _ := s.Models().AddProfile(ctx, modelInput())
	b, _ := s.Models().AddProfile(ctx, modelInput())
	for _, p := range []llm.Profile{a, b} {
		if _, err := control(s).Test(ctx, p.ProfileID); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i, store := range []*Store{s, other} {
		wg.Add(1)
		go func(i int, store *Store) {
			defer wg.Done()
			results <- control(store).Select(ctx, []string{a.ProfileID, b.ProfileID}[i], 1, 1)
		}(i, store)
	}
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for err := range results {
		if err == nil {
			wins++
		} else if errors.Is(err, llm.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("wins=%d conflicts=%d", wins, conflicts)
	}
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM model_profile WHERE status='ready'`).Scan(&n); err != nil || n != 1 {
		t.Fatal("loser profile committed", n, err)
	}
}

func TestFailedRetestRevokesOnlyCurrentProof(t *testing.T) {
	ctx := context.Background()
	s := openModels(t)
	c := control(s)
	p, err := s.Models().AddProfile(ctx, modelInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Test(ctx, p.ProfileID); err != nil {
		t.Fatal(err)
	}
	if err := c.Select(ctx, p.ProfileID, 1, 1); err != nil {
		t.Fatal(err)
	}
	c.Probe = probeFixture{err: llm.ModelIncompatible}
	if _, err := c.Test(ctx, p.ProfileID); !errors.Is(err, llm.ModelIncompatible) {
		t.Fatal(err)
	}
	stored, err := s.Models().Profile(ctx, p.ProfileID)
	if err != nil || stored.Status != "invalid" || stored.Version != 3 || stored.VerifiedAt != nil {
		t.Fatal("failed retest retained ready status", stored, err)
	}
	d, err := s.Diagnostics(ctx)
	if err != nil || d.Readiness != "degraded" || d.ConfigVersion != 3 || c.Available(ctx, p.ProfileID) {
		t.Fatal("failed retest retained ingress", d, err)
	}
	c = control(s)
	if _, err := c.Test(ctx, p.ProfileID); err != nil {
		t.Fatal(err)
	}
	if err := s.Models().InvalidateProbe(ctx, p.ProfileID, 2, "invalid"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Models().Probe(ctx, p.ProfileID); err != nil {
		t.Fatal("old failure erased current proof", err)
	}
	if err := c.Select(ctx, p.ProfileID, 3, 3); err != nil {
		t.Fatal("repaired profile cannot be selected", err)
	}
}

type probeFunc func(context.Context, llm.Profile) (llm.ProbeResult, error)

func (f probeFunc) Run(ctx context.Context, p llm.Profile) (llm.ProbeResult, error) {
	return f(ctx, p)
}

func TestRetestCancellationUsesCallerContext(t *testing.T) {
	for _, callerCancelled := range []bool{false, true} {
		for _, cause := range []error{llm.GenerationCancelled, errors.Join(llm.GenerationCancelled, llm.ProviderUnavailable)} {
			t.Run(fmt.Sprintf("caller_cancelled=%v/cleanup_failed=%v", callerCancelled, errors.Is(cause, llm.ProviderUnavailable)), func(t *testing.T) {
				ctx := context.Background()
				s := openModels(t)
				c := control(s)
				p, err := s.Models().AddProfile(ctx, modelInput())
				if err != nil {
					t.Fatal(err)
				}
				if _, err = c.Test(ctx, p.ProfileID); err != nil {
					t.Fatal(err)
				}
				if err = c.Select(ctx, p.ProfileID, 1, 1); err != nil {
					t.Fatal(err)
				}
				before, err := s.Models().Profile(ctx, p.ProfileID)
				if err != nil {
					t.Fatal(err)
				}
				configBefore, err := s.Diagnostics(ctx)
				if err != nil {
					t.Fatal(err)
				}
				proofBefore, err := s.Models().Probe(ctx, p.ProfileID)
				if err != nil {
					t.Fatal(err)
				}
				events := countRows(t, s, "event_outbox")
				caller, cancel := context.WithCancel(ctx)
				defer cancel()
				c.Probe = probeFunc(func(context.Context, llm.Profile) (llm.ProbeResult, error) {
					if callerCancelled {
						cancel()
					}
					return llm.ProbeResult{}, cause
				})
				if _, err = c.Test(caller, p.ProfileID); !errors.Is(err, cause) {
					t.Fatal("lost cancellation error contract", err)
				}
				after, err := s.Models().Profile(ctx, p.ProfileID)
				if err != nil {
					t.Fatal(err)
				}
				configAfter, err := s.Diagnostics(ctx)
				if err != nil {
					t.Fatal(err)
				}
				proofAfter, proofErr := s.Models().Probe(ctx, p.ProfileID)
				if callerCancelled {
					if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(configBefore, configAfter) || proofErr != nil || !reflect.DeepEqual(proofBefore, proofAfter) || countRows(t, s, "event_outbox") != events || !c.Available(ctx, p.ProfileID) {
						t.Fatal("caller cancellation changed prior state", after, configAfter, proofErr)
					}
				} else if after.Status != "unavailable" || after.Version != before.Version+1 || after.VerifiedAt != nil || configAfter.Readiness != "degraded" || configAfter.ConfigVersion != configBefore.ConfigVersion+1 || !errors.Is(proofErr, llm.ErrNotFound) || countRows(t, s, "event_outbox") != events+2 || c.Available(ctx, p.ProfileID) {
					t.Fatal("internal cancellation failure retained readiness proof", after, configAfter, proofErr)
				}
			})
		}
	}
}

func TestProbeInvalidationAuthAndMissingSecrets(t *testing.T) {
	ctx := context.Background()
	s := openModels(t)
	c := control(s)
	p, _ := s.Models().AddProfile(ctx, modelInput())
	c.Probe = probeFixture{result: llm.ProbeResult{Streaming: true}}
	if _, err := c.Test(ctx, p.ProfileID); !errors.Is(err, llm.ModelIncompatible) {
		t.Fatal("partial probe accepted", err)
	}
	if countRows(t, s, "model_probe") != 0 {
		t.Fatal("failed probe persisted")
	}
	c = control(s)
	if _, err := c.Test(ctx, p.ProfileID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Models().UpdateProfile(ctx, p.ProfileID, 1, modelInput()); err != nil {
		t.Fatal(err)
	}
	if err := c.Select(ctx, p.ProfileID, 2, 1); !errors.Is(err, llm.ErrUnverified) {
		t.Fatal("stale probe accepted", err)
	}
	secrets := memory.New()
	a, err := llm.RegisterAuth(ctx, s.Models(), secrets, llm.AuthInput{Provider: "local", AuthKind: "api_key"}, []byte("SYNTHETIC_CREDENTIAL"))
	if err != nil {
		t.Fatal(err)
	}
	input := modelInput()
	input.AuthConnectionID = a.ConnectionID
	authed, err := s.Models().AddProfile(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Test(ctx, authed.ProfileID); !errors.Is(err, llm.AuthRequired) {
		t.Fatal("missing SecretStore accepted", err)
	}
	c.Secrets = secrets
	if _, err := c.Test(ctx, authed.ProfileID); err != nil {
		t.Fatal(err)
	}
	if err := s.Models().UpdateAuthState(ctx, a.ConnectionID, 1, "logged_out", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Select(ctx, authed.ProfileID, 1, 1); !errors.Is(err, llm.AuthRequired) {
		t.Fatal("logged out accepted", err)
	}
	if err := s.Models().UpdateAuthState(ctx, a.ConnectionID, 2, "connected", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Select(ctx, authed.ProfileID, 1, 1); !errors.Is(err, llm.ErrUnverified) {
		t.Fatal("auth version drift accepted", err)
	}
	if _, err := c.Test(ctx, authed.ProfileID); err != nil {
		t.Fatal(err)
	}
	if err := secrets.Delete(ctx, a.CredentialRef); err != nil {
		t.Fatal(err)
	}
	if err := c.Select(ctx, authed.ProfileID, 1, 1); !errors.Is(err, llm.AuthRequired) {
		t.Fatal("missing credential accepted", err)
	}
	if err := secrets.Put(ctx, a.CredentialRef, []byte("SYNTHETIC_CREDENTIAL")); err != nil {
		t.Fatal(err)
	}
	if err := c.Select(ctx, authed.ProfileID, 1, 1); err != nil {
		t.Fatal(err)
	}
	if !c.Available(ctx, authed.ProfileID) {
		t.Fatal("authenticated profile not ready")
	}
	if err := secrets.Delete(ctx, a.CredentialRef); err != nil {
		t.Fatal(err)
	}
	if c.Available(ctx, authed.ProfileID) {
		t.Fatal("missing active credential did not close ingress")
	}
}

func TestV3MigrationRollbackPreservesMetadata(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := migrate(ctx, db, migrations[:3]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO agent_config VALUES(1,'prior-agent',NULL,'setup_required',9,100,200)`); err != nil {
		t.Fatal(err)
	}
	legacy := &Store{db: db, path: filepath.Join(root, "state.db")}
	a, err := llm.RegisterAuth(ctx, legacy.Models(), memory.New(), llm.AuthInput{Provider: "local", AuthKind: "api_key"}, []byte("SYNTHETIC_MIGRATION_SECRET"))
	if err != nil {
		t.Fatal(err)
	}
	input := modelInput()
	input.AuthConnectionID = a.ConnectionID
	p, err := legacy.Models().AddProfile(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	p = markReady(t, legacy, p)
	var snapshot llm.Snapshot
	if err := legacy.Transaction(ctx, func(tx *sql.Tx) error {
		var err error
		snapshot, err = legacy.Models().FreezeProfile(ctx, tx, p.ProfileID, p.Version)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	events := countRows(t, legacy, "event_outbox")
	broken := append(append([]string{}, migrations[:3]...), providerSQL+`; INVALID SQL`)
	if err := migrate(ctx, db, broken); err == nil {
		t.Fatal("broken migration accepted")
	}
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 3 {
		t.Fatal("failed migration changed schema", version, err)
	}
	var config int64
	if err := db.QueryRow(`SELECT config_version FROM agent_config WHERE agent_id='prior-agent'`).Scan(&config); err != nil || config != 9 {
		t.Fatal("rollback lost config", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d, err := s.Diagnostics(ctx)
	if err != nil || d.SchemaVersion != 17 || d.AgentID != "prior-agent" || d.ConfigVersion != 9 || d.Readiness != "setup_required" {
		t.Fatal("v3 upgrade changed identity", d, err)
	}
	stored, err := s.Models().Profile(ctx, p.ProfileID)
	if err != nil || !reflect.DeepEqual(stored, p) {
		t.Fatal("v3 profile changed", stored, err)
	}
	storedAuth, err := s.Models().Auth(ctx, a.ConnectionID)
	if err != nil || !reflect.DeepEqual(storedAuth, a) {
		t.Fatal("v3 auth changed", err)
	}
	storedSnapshot, err := s.Models().Snapshot(ctx, snapshot.SnapshotID)
	if err != nil || !reflect.DeepEqual(storedSnapshot, snapshot) || countRows(t, s, "event_outbox") != events {
		t.Fatal("v3 snapshot/events changed", err)
	}
}

func TestV3OverLimitMetadataRemainsReadable(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := migrate(ctx, db, migrations[:3]); err != nil {
		t.Fatal(err)
	}
	legacy := &Store{db: db, path: filepath.Join(root, "state.db")}
	p, err := legacy.Models().AddProfile(ctx, modelInput())
	if err != nil {
		t.Fatal(err)
	}
	// These values satisfy the v3 schema and the released v3 CLI validation.
	if _, err := db.Exec(`UPDATE model_profile SET context_window=2097152,max_output_tokens=131072,version=version+1 WHERE profile_id=?`, p.ProfileID); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	profiles, err := s.Models().ListProfiles(ctx)
	if err != nil || len(profiles) != 1 {
		t.Fatalf("legacy diagnostic list failed: %+v %v", profiles, err)
	}
	stored, err := s.Models().Profile(ctx, p.ProfileID)
	if err != nil || stored.ContextWindow != 2097152 || stored.MaxOutputTokens != 131072 || stored.Version != 2 {
		t.Fatal("legacy metadata rewritten or unreadable", stored, err)
	}
	c := control(s)
	if _, err := c.Test(ctx, p.ProfileID); err == nil {
		t.Fatal("over-limit profile passed activation Probe")
	}
	if err := c.Select(ctx, p.ProfileID, 2, 1); err == nil {
		t.Fatal("over-limit profile selected")
	}
	d, err := s.Diagnostics(ctx)
	if err != nil || d.Readiness != "setup_required" || d.AgentIngressEnabled {
		t.Fatal("legacy profile enabled ingress", d, err)
	}
}
