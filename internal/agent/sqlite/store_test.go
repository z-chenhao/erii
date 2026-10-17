package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/z-chenhao/erii/internal/agent"
)

func TestFreshAndRepeatedStartupPreserveConfiguration(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "state with # and ?")
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.Diagnostics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id, err := uuid.Parse(d.AgentID)
	if err != nil || id.Version() != 7 {
		t.Fatalf("agent identity: %q (%v)", d.AgentID, err)
	}
	if d.Readiness != agent.SetupRequired || d.AgentIngressEnabled || d.ActiveModelProfileID != nil || d.ConfigVersion != 1 || d.SchemaVersion != 17 {
		t.Fatalf("diagnostics: %+v", d)
	}
	var created, updated int64
	if err := s.db.QueryRow("SELECT created_at, updated_at FROM agent_config").Scan(&created, &updated); err != nil {
		t.Fatal(err)
	}
	if created != updated || time.Since(time.UnixMicro(created)) < 0 || time.Since(time.UnixMicro(created)) > time.Minute {
		t.Fatalf("expected UTC microseconds, got %d/%d", created, updated)
	}
	// Force replacement connections so PRAGMA settings cannot rely on one initial connection.
	s.db.SetConnMaxLifetime(time.Nanosecond)
	for pragma, want := range map[string]string{"journal_mode": "wal", "foreign_keys": "1", "busy_timeout": "5000", "user_version": "17"} {
		var got string
		if err := s.db.QueryRow("PRAGMA " + pragma).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s = %q, want %q", pragma, got, want)
		}
	}
	for _, query := range []string{
		"UPDATE agent_config SET active_model_profile_id = 'unverified'",
		"UPDATE agent_config SET setup_status = 'ready'",
		"UPDATE agent_config SET config_version = 0",
		"UPDATE agent_config SET updated_at = 0",
		"UPDATE agent_config SET singleton = 2",
	} {
		if _, err := s.db.Exec(query); err == nil {
			t.Fatalf("accepted invalid state: %s", query)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	again, err := s.Diagnostics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d, again) {
		t.Fatalf("startup changed state: %+v -> %+v", d, again)
	}
	var newCreated, newUpdated int64
	if err := s.db.QueryRow("SELECT created_at, updated_at FROM agent_config").Scan(&newCreated, &newUpdated); err != nil {
		t.Fatal(err)
	}
	if created != newCreated || updated != newUpdated {
		t.Fatal("startup changed timestamps")
	}
}

func TestMigrationForwardRollbackAndFutureVersion(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	// Synthetic prior versions exercise the migration mechanism without adding
	// speculative production tables or claiming a previously released schema.
	v1 := []string{"CREATE TABLE prior (value TEXT); INSERT INTO prior VALUES ('kept');"}
	if err := migrate(ctx, db, v1); err != nil {
		t.Fatal(err)
	}
	failed := append(append([]string{}, v1...), "CREATE TABLE rolled_back (id INTEGER); INVALID SQL")
	if err := migrate(ctx, db, failed); err == nil {
		t.Fatal("accepted broken migration")
	}
	var version, count int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("failed migration changed version to %d", version)
	}
	if err := db.QueryRow("SELECT count(*) FROM sqlite_schema WHERE name = 'rolled_back'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed DDL was not rolled back")
	}
	v2 := append(append([]string{}, v1...), "ALTER TABLE prior ADD COLUMN revision INTEGER NOT NULL DEFAULT 1")
	if err := migrate(ctx, db, v2); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, v2); err != nil {
		t.Fatal(err)
	}
	var value string
	var revision int
	if err := db.QueryRow("SELECT value, revision FROM prior").Scan(&value, &revision); err != nil {
		t.Fatal(err)
	}
	if value != "kept" || revision != 1 {
		t.Fatal("migration did not preserve prior row")
	}
	if err := migrate(ctx, db, v1); err == nil || !strings.Contains(err.Error(), "unsupported database schema version 2") {
		t.Fatalf("older binary accepted newer schema: %v", err)
	}
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatal("rejected downgrade changed schema version")
	}
}

func TestOpenRejectsUnsupportedSchemaAndUnwritableRoot(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), dir); err == nil {
		t.Fatal("accepted future schema")
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), blocked); err == nil {
		t.Fatal("accepted file as data root")
	}
}

func TestConcurrentBootstrapHasOneIdentity(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	type result struct {
		diagnostics agent.Diagnostics
		err         error
	}
	results := make(chan result, 4)
	start := make(chan struct{})
	for range 4 {
		go func() {
			<-start
			s, err := Open(ctx, dir)
			if err != nil {
				results <- result{err: err}
				return
			}
			d, err := s.Diagnostics(ctx)
			err = errors.Join(err, s.Close())
			results <- result{d, err}
		}()
	}
	close(start)
	var first agent.Diagnostics
	for i := range 4 {
		r := <-results
		if r.err != nil {
			t.Errorf("startup %d: %v", i, r.err)
			continue
		}
		if first.AgentID == "" {
			first = r.diagnostics
		}
		if !reflect.DeepEqual(first, r.diagnostics) {
			t.Errorf("concurrent startups disagree: %+v, %+v", first, r.diagnostics)
		}
	}
}

func TestRealV1UpgradePreservesIdentityWithoutRetroactiveEvents(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, migrations[:1]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO agent_config VALUES (1,'existing-agent',NULL,'setup_required',1,100,100)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d, err := s.Diagnostics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d.AgentID != "existing-agent" || d.SchemaVersion != 17 || d.Readiness != "setup_required" {
		t.Fatalf("upgrade: %+v", d)
	}
	var created, events int
	if err := s.db.QueryRow(`SELECT created_at FROM agent_config`).Scan(&created); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM event_outbox`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if created != 100 || events != 0 {
		t.Fatal("upgrade rewrote domain state or fabricated an event")
	}
}

func TestBootstrapAndEventRollBackTogether(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := migrate(ctx, db, migrations); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_event BEFORE INSERT ON event_outbox BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: db}
	if err := s.bootstrap(ctx); err == nil {
		t.Fatal("expected event persistence fault")
	}
	var rows int
	if err := db.QueryRow(`SELECT count(*) FROM agent_config`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatal("domain bootstrap committed without event")
	}
}
