package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/z-chenhao/erii/internal/llm"
	"github.com/z-chenhao/erii/internal/llm/account"
	"github.com/z-chenhao/erii/internal/llm/memory"
)

type accountPeer struct {
	count         atomic.Int32
	fail          bool
	hold, release chan struct{}
	client        string
}

func (p *accountPeer) AccountStart(context.Context, string, *llm.AccountRegistration, *llm.OAuthCredential) (llm.AccountLogin, error) {
	c := llm.OAuthCredential{ClientID: "fixture-client", SubjectHash: strings.Repeat("a", 64), AccessToken: "fixture-access-marker", RefreshToken: "fixture-refresh-marker", IDToken: "fixture-id-marker", Scopes: []string{"chatgpt.tokens.use.direct"}, ExpiresAt: time.Now().Add(time.Hour)}
	if p.client != "" {
		c.ClientID = p.client
	}
	return llm.AccountLogin{Status: "connected", Credential: &c}, nil
}

type trackedSecrets struct {
	llm.SecretStore
	ref        string
	cancel     context.CancelFunc
	failDelete bool
}

func (s *trackedSecrets) Put(ctx context.Context, ref string, value []byte) error {
	s.ref = ref
	err := s.SecretStore.Put(ctx, ref, value)
	if s.cancel != nil {
		s.cancel()
	}
	return err
}
func (s *trackedSecrets) Delete(ctx context.Context, ref string) error {
	if s.failDelete {
		return errors.New("unsafe credential backend text")
	}
	return s.SecretStore.Delete(ctx, ref)
}
func TestAccountRegistrationFailureAndLogoutCleanupRecovery(t *testing.T) {
	for _, mode := range []string{"commit", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			s := openModels(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			base := memory.New()
			secrets := &trackedSecrets{SecretStore: base}
			m := &account.Manager{Store: s.Models(), Secrets: secrets, Provider: &accountPeer{}, LockRoot: filepath.Join(filepath.Dir(s.path), "locks")}
			if mode == "commit" {
				if _, err := s.db.Exec(`CREATE TRIGGER fail_registration BEFORE INSERT ON account_registration BEGIN SELECT RAISE(ABORT,'unsafe'); END`); err != nil {
					t.Fatal(err)
				}
			} else {
				secrets.cancel = cancel
			}
			if _, err := m.Login(ctx, "", nil); err == nil {
				t.Fatal("accepted failed credential metadata commit")
			}
			if _, err := base.Get(context.Background(), secrets.ref); err == nil {
				t.Fatal("new unreferenced credential not cleaned")
			}
			if countRows(t, s, "auth_connection") != 0 || countRows(t, s, "account_registration") != 0 {
				t.Fatal("partial registration")
			}
		})
	}
	s, m, _, base, a := accountManager(t)
	ctx := context.Background()
	tracked := &trackedSecrets{SecretStore: base, failDelete: true}
	m.Secrets = tracked
	if _, err := m.Logout(ctx, a.ConnectionID); !errors.Is(err, llm.ErrSecretCleanup) || strings.Contains(err.Error(), "unsafe") {
		t.Fatal("unsanitized deletion failure", err)
	}
	current, _ := s.Models().Auth(ctx, a.ConnectionID)
	if current.Status != "logged_out" {
		t.Fatal("failed deletion left admission open")
	}
	tracked.failDelete = false
	if err := m.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := base.Get(ctx, a.CredentialRef); err == nil {
		t.Fatal("restart retained logged-out tokens")
	}
}
func TestAccountIsolationAndLegalTransitions(t *testing.T) {
	s, m, peer, _, a := accountManager(t)
	ctx := context.Background()
	peer.client = "other-workspace"
	b, err := m.Login(ctx, "", nil)
	if err != nil || b.ConnectionID == a.ConnectionID || b.WorkspaceLabel == a.WorkspaceLabel {
		t.Fatal("registrations combined", err)
	}
	if _, err = m.Logout(ctx, a.ConnectionID); err != nil {
		t.Fatal(err)
	}
	if _, _, err = m.Ensure(ctx, b.ConnectionID); err != nil {
		t.Fatal("logout crossed account scope", err)
	}
	old, _ := s.Models().Auth(ctx, a.ConnectionID)
	if err = s.Models().AccountState(ctx, old, "refreshing", nil); err == nil {
		t.Fatal("logged_out -> refreshing accepted")
	}
	if _, err = s.db.Exec(`UPDATE account_registration SET client_id='replacement',auth_epoch=auth_epoch+1 WHERE connection_id=?`, b.ConnectionID); err == nil {
		t.Fatal("changed stable account identity")
	}
	device, err := s.Models().DeviceID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Models().DeviceID(ctx)
	if err != nil || device != again {
		t.Fatal("unstable installation ID")
	}
}

func TestReauthorizationCommitFailureRemovesReplacement(t *testing.T) {
	s, m, _, secrets, a := accountManager(t)
	ctx := context.Background()
	if _, err := m.Logout(ctx, a.ConnectionID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_reauthorization BEFORE UPDATE ON auth_connection WHEN NEW.status='connected' BEGIN SELECT RAISE(ABORT,'fault'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Login(ctx, a.ConnectionID, nil); err != llm.ErrStorage {
		t.Fatal("accepted failed reauthorization", err)
	}
	if _, err := secrets.Get(ctx, a.CredentialRef); err == nil {
		t.Fatal("failed reauthorization retained replacement")
	}
	current, _ := s.Models().Auth(ctx, a.ConnectionID)
	if current.Status != "logged_out" {
		t.Fatal("failed commit reopened admission")
	}
}
func (*accountPeer) AccountPoll(context.Context) (llm.AccountLogin, error) {
	return llm.AccountLogin{}, llm.AuthRequired
}
func (*accountPeer) AccountCancel(context.Context) error { return nil }
func (p *accountPeer) AccountRefresh(_ context.Context, c llm.OAuthCredential) (llm.OAuthCredential, error) {
	p.count.Add(1)
	if p.hold != nil {
		close(p.hold)
		<-p.release
	}
	if p.fail {
		return c, llm.AuthRequired
	}
	c.AccessToken = "rotated-access-marker"
	c.RefreshToken = "rotated-refresh-marker"
	c.ExpiresAt = time.Now().Add(time.Hour)
	return c, nil
}
func (*accountPeer) AccountLogout(context.Context, string, *llm.OAuthCredential) (bool, error) {
	return true, nil
}
func accountManager(t *testing.T) (*Store, *account.Manager, *accountPeer, *memory.SecretStore, llm.AuthConnection) {
	t.Helper()
	s := openModels(t)
	secrets := memory.New()
	peer := &accountPeer{}
	m := &account.Manager{Store: s.Models(), Secrets: secrets, Provider: peer, LockRoot: filepath.Join(filepath.Dir(s.path), "locks")}
	a, err := m.Login(context.Background(), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return s, m, peer, secrets, a
}
func expireAccount(t *testing.T, secrets llm.SecretStore, a llm.AuthConnection) {
	t.Helper()
	data, err := secrets.Get(context.Background(), a.CredentialRef)
	if err != nil {
		t.Fatal(err)
	}
	var c llm.OAuthCredential
	if json.Unmarshal(data, &c) != nil {
		t.Fatal("decode")
	}
	c.ExpiresAt = time.Now().Add(-time.Minute)
	data, _ = json.Marshal(c)
	if secrets.Put(context.Background(), a.CredentialRef, data) != nil {
		t.Fatal("put")
	}
}
func activateAccount(t *testing.T, s *Store, m *account.Manager, a llm.AuthConnection) llm.Profile {
	t.Helper()
	ctx := context.Background()
	input := modelInput()
	input.Endpoint = ""
	input.ProviderKind = "openai_account"
	input.WireAPI = "openai_responses"
	input.AuthConnectionID = a.ConnectionID
	p, err := s.Models().AddProfile(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	c := control(s)
	c.Secrets = m.Secrets
	c.AccountAuth = func(ctx context.Context, id string) (int64, error) {
		a, _, err := m.Ensure(ctx, id)
		return a.Version, err
	}
	if _, err = c.Test(ctx, p.ProfileID); err != nil {
		t.Fatal(err)
	}
	if err = c.Select(ctx, p.ProfileID, 1, 1); err != nil {
		t.Fatal(err)
	}
	p, err = s.Models().Profile(ctx, p.ProfileID)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestAccountSingleFlightAcrossManagersAndLogout(t *testing.T) {
	s, m, peer, secrets, a := accountManager(t)
	ctx := context.Background()
	p := activateAccount(t, s, m, a)
	expireAccount(t, secrets, a)
	other, err := Open(ctx, filepath.Dir(s.path))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	m2 := &account.Manager{Store: other.Models(), Secrets: secrets, Provider: peer, LockRoot: m.LockRoot}
	var wg sync.WaitGroup
	errs := make(chan error, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			manager := m
			if i%2 == 1 {
				manager = m2
			}
			_, _, err := manager.Ensure(ctx, a.ConnectionID)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if peer.count.Load() != 1 {
		t.Fatalf("refresh count %d", peer.count.Load())
	}
	proof, err := s.Models().Probe(ctx, p.ProfileID)
	auth, _ := s.Models().Auth(ctx, a.ConnectionID)
	if err != nil || proof.AuthVersion != auth.Version || auth.Status != "connected" {
		t.Fatal("refresh lost proof", err)
	}
	if confirmed, err := m2.Logout(ctx, a.ConnectionID); err != nil || !confirmed {
		t.Fatal(err)
	}
	if _, _, err = m.Ensure(ctx, a.ConnectionID); !errors.Is(err, llm.AuthRequired) {
		t.Fatal("logout admitted", err)
	}
	if _, err = secrets.Get(ctx, a.CredentialRef); err == nil {
		t.Fatal("logout retained token")
	}
	after, _ := s.Models().Profile(ctx, p.ProfileID)
	d, _ := s.Diagnostics(ctx)
	if after.Status != "reauth_required" || d.Readiness != "degraded" || countRows(t, s, "model_probe") != 0 {
		t.Fatal("logout failed to revoke readiness")
	}
	if _, err = m.Logout(ctx, a.ConnectionID); err != nil {
		t.Fatal("idempotent logout", err)
	}
	newAuth, err := m.Login(ctx, a.ConnectionID, nil)
	if err != nil || newAuth.ConnectionID != a.ConnectionID {
		t.Fatal("reauthorization lost identity", err)
	}
}
func TestAccountRotationFailureAndInterruptedRefresh(t *testing.T) {
	for _, mode := range []string{"provider", "secret-write", "interrupted"} {
		t.Run(mode, func(t *testing.T) {
			s, m, peer, secrets, a := accountManager(t)
			ctx := context.Background()
			p := activateAccount(t, s, m, a)
			expireAccount(t, secrets, a)
			if mode == "provider" {
				peer.fail = true
			}
			if mode == "secret-write" {
				m.Secrets = failingPut{secrets}
			}
			if mode == "interrupted" {
				if s.Models().AccountState(ctx, a, "refreshing", nil) != nil {
					t.Fatal("state")
				}
				if err := m.Recover(ctx); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, _, err := m.Ensure(ctx, a.ConnectionID); err == nil {
					t.Fatal("accepted failed rotation")
				}
			}
			after, _ := s.Models().Auth(ctx, a.ConnectionID)
			profile, _ := s.Models().Profile(ctx, p.ProfileID)
			if after.Status != "reauth_required" || profile.Status != "reauth_required" {
				t.Fatal("not reauth")
			}
			if _, err := secrets.Get(ctx, a.CredentialRef); err == nil {
				t.Fatal("failed rotation retained secret")
			}
			if _, _, err := m.Ensure(ctx, a.ConnectionID); err == nil {
				t.Fatal("retried invalid credential")
			}
			want := int32(1)
			if mode == "interrupted" {
				want = 0
			}
			if peer.count.Load() != want {
				t.Fatal("rotating call retried")
			}
		})
	}
}

type failingPut struct{ llm.SecretStore }

func (failingPut) Put(context.Context, string, []byte) error {
	return errors.New("unsafe provider credential material")
}
func TestLogoutSerializesWithRefreshAndRejectsStaleMutation(t *testing.T) {
	s, m, peer, secrets, a := accountManager(t)
	ctx := context.Background()
	expireAccount(t, secrets, a)
	peer.hold = make(chan struct{})
	peer.release = make(chan struct{})
	result := make(chan error, 2)
	go func() { _, _, err := m.Ensure(ctx, a.ConnectionID); result <- err }()
	<-peer.hold
	go func() { _, err := m.Logout(ctx, a.ConnectionID); result <- err }()
	close(peer.release)
	for i := 0; i < 2; i++ {
		if err := <-result; err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Models().AccountState(ctx, a, "connected", nil); !errors.Is(err, llm.ErrConflict) {
		t.Fatal("stale version revived credentials", err)
	}
	current, _ := s.Models().Auth(ctx, a.ConnectionID)
	if current.Status != "logged_out" {
		t.Fatal("late refresh won")
	}
}

func TestAccountEpochRejectsOldResultsAfterReauthorization(t *testing.T) {
	_, m, _, secrets, a := accountManager(t)
	ctx := context.Background()
	resolve := m.Resolver(nil)
	snapshot := llm.Snapshot{ProviderKind: "openai_account", WireAPI: "openai_responses", AuthConnectionID: a.ConnectionID}
	old, err := resolve(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	expireAccount(t, secrets, a)
	if _, _, err = m.Ensure(ctx, a.ConnectionID); err != nil {
		t.Fatal(err)
	}
	if err = m.LoseVersion(ctx, a.ConnectionID, old.AuthVersion); err != nil {
		t.Fatal(err)
	}
	if old.Guard(ctx) != nil {
		t.Fatal("old token error revoked successfully refreshed account")
	}
	if _, err = m.Logout(ctx, a.ConnectionID); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Login(ctx, a.ConnectionID, nil); err != nil {
		t.Fatal(err)
	}
	if old.Guard(ctx) != llm.AuthRequired {
		t.Fatal("old generation admitted after reauthorization")
	}
	current, err := resolve(ctx, snapshot)
	if err != nil || current.AuthEpoch == old.AuthEpoch || current.Guard(ctx) != nil {
		t.Fatal("new generation not usable", err)
	}
	var cause error
	observer := m.Observer(func(_ context.Context, _ llm.GenerationRequest, result llm.GenerationResult, _ time.Duration, err error) error {
		cause = err
		if result.Text != "" {
			t.Fatal("old result persisted")
		}
		return nil
	})
	if err = observer(ctx, llm.GenerationRequest{Snapshot: snapshot}, old, llm.GenerationResult{Text: "late result"}, time.Second, nil); err != llm.AuthRequired || cause != llm.AuthRequired {
		t.Fatal("final result escaped epoch fence", err)
	}
}
func TestAccountStateTransactionRollbackAndSecretRedaction(t *testing.T) {
	s, m, _, _, a := accountManager(t)
	ctx := context.Background()
	activateAccount(t, s, m, a)
	if _, err := s.db.Exec(`CREATE TRIGGER fail_auth_event BEFORE INSERT ON event_outbox WHEN NEW.type='ai.erii.auth_connection.updated.v1' BEGIN SELECT RAISE(ABORT,'secret-marker'); END`); err != nil {
		t.Fatal(err)
	}
	before := countRows(t, s, "event_outbox")
	if err := s.Models().AccountState(ctx, a, "logged_out", nil); err != llm.ErrStorage {
		t.Fatal(err)
	}
	after, _ := s.Models().Auth(ctx, a.ConnectionID)
	if after.Status != "connected" || before != countRows(t, s, "event_outbox") || countRows(t, s, "model_probe") != 1 {
		t.Fatal("auth transaction partially committed")
	}
	encoded, _ := json.Marshal(after)
	if strings.Contains(string(encoded), "credential:") || strings.Contains(string(encoded), "fixture-access") {
		t.Fatal("secret in UI metadata")
	}
}
func TestV5AccountMigrationPreservesStateAndRollsBack(t *testing.T) {
	for _, broken := range []bool{false, true} {
		t.Run(map[bool]string{false: "forward", true: "rollback"}[broken], func(t *testing.T) {
			db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "prior.db")+"?_pragma=foreign_keys(1)&_txlock=immediate")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			ctx := context.Background()
			if err = migrate(ctx, db, migrations[:5]); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(`CREATE TABLE preserved(value TEXT); INSERT INTO preserved VALUES('kept');`); err != nil {
				t.Fatal(err)
			}
			if broken {
				if _, err = db.Exec(`CREATE TABLE account_registration(conflict TEXT)`); err != nil {
					t.Fatal(err)
				}
			}
			err = migrate(ctx, db, migrations)
			if (err != nil) != broken {
				t.Fatal("migration outcome", err)
			}
			var v int
			_ = db.QueryRow(`PRAGMA user_version`).Scan(&v)
			if broken && v != 5 || !broken && v != 17 {
				t.Fatal("schema version", v)
			}
			var value string
			if db.QueryRow(`SELECT value FROM preserved`).Scan(&value) != nil || value != "kept" {
				t.Fatal("prior data lost")
			}
			if broken {
				var n int
				_ = db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name='installation_auth'`).Scan(&n)
				if n != 0 {
					t.Fatal("partial DDL")
				}
			} else {
				if err = migrate(ctx, db, migrations); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
