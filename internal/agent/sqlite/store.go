// Package sqlite owns the foundation's state.db connection and forward migrations.
package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/z-chenhao/erii/internal/agent"
	artifactsqlite "github.com/z-chenhao/erii/internal/artifact/sqlite"
	"github.com/z-chenhao/erii/internal/event"
	eventsqlite "github.com/z-chenhao/erii/internal/event/sqlite"
	llmsqlite "github.com/z-chenhao/erii/internal/llm/sqlite"
	sqlitedriver "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

//go:embed migrations/001_bootstrap.sql
var bootstrapSQL string

//go:embed migrations/002_events.sql
var eventsSQL string

//go:embed migrations/003_models.sql
var modelsSQL string

//go:embed migrations/004_provider.sql
var providerSQL string

//go:embed migrations/005_local_provider.sql
var localProviderSQL string

//go:embed migrations/006_account_auth.sql
var accountAuthSQL string

//go:embed migrations/007_main_loop.sql
var mainLoopSQL string

//go:embed migrations/008_tasks.sql
var tasksSQL string

//go:embed migrations/009_task_control.sql
var taskControlSQL string

//go:embed migrations/010_tool_policy.sql
var toolPolicySQL string

//go:embed migrations/011_artifacts.sql
var artifactsSQL string

//go:embed migrations/012_delivery.sql
var deliverySQL string

//go:embed migrations/013_children.sql
var childrenSQL string

//go:embed migrations/014_scheduler.sql
var schedulerSQL string

//go:embed migrations/015_spend.sql
var spendSQL string

//go:embed migrations/016_codemode.sql
var codemodeSQL string

//go:embed migrations/017_code_recovery.sql
var codeRecoverySQL string

var migrations = []string{bootstrapSQL, eventsSQL, modelsSQL, providerSQL, localProviderSQL, accountAuthSQL, mainLoopSQL, tasksSQL, taskControlSQL, toolPolicySQL, artifactsSQL, deliverySQL, childrenSQL, schedulerSQL, spendSQL, codemodeSQL, codeRecoverySQL}

type Store struct {
	scheduler agent.SchedulerLimits
	db        *sql.DB
	path      string
	artifacts *artifactsqlite.Store
	// MainBoundary is explicit fault injection; production composition leaves it nil.
	MainBoundary func(string)
}

func (s *Store) boundary(name string) {
	if s.MainBoundary != nil {
		s.MainBoundary(name)
	}
}

func Open(ctx context.Context, dataRoot string) (*Store, error) {
	if !filepath.IsAbs(dataRoot) {
		return nil, errors.New("database data root must be absolute")
	}
	if err := os.MkdirAll(dataRoot, 0700); err != nil {
		return nil, fmt.Errorf("create data root: %w", err)
	}
	path := filepath.Join(dataRoot, "state.db")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open state database file: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	// DSN settings apply to every connection, including replacements.
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Set("_txlock", "immediate")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, fmt.Errorf("open SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := connect(ctx, db); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	if err := migrate(ctx, db, migrations); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	s := &Store{db: db, path: path}
	if err := s.bootstrap(ctx); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	if _, err := s.Diagnostics(ctx); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	s.artifacts, err = artifactsqlite.New(db, dataRoot)
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	s.artifacts.Boundary = s.boundary
	if _, err = s.artifacts.Recover(ctx); err != nil {
		return nil, errors.Join(err, s.artifacts.Close(), db.Close())
	}
	return s, nil
}

// Concurrent first-open journal-mode changes can return SQLITE_BUSY without
// invoking SQLite's busy handler. Retry only connection initialization, whose
// PRAGMAs are idempotent; never replay migration/commit errors here.
func connect(ctx context.Context, db *sql.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for {
		err := db.PingContext(ctx)
		if err == nil {
			return nil
		}
		var sqliteErr *sqlitedriver.Error
		if !errors.As(err, &sqliteErr) || sqliteErr.Code()&0xff != sqlite3.SQLITE_BUSY {
			return fmt.Errorf("initialize SQLite connection: %w", err)
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("initialize SQLite connection: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

// The immediate transaction serializes the version check and all DDL, including
// across processes. Failure leaves both schema and user_version unchanged.
func migrate(ctx context.Context, db *sql.DB, scripts []string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version < 0 || version > len(scripts) {
		return fmt.Errorf("unsupported database schema version %d (maximum %d)", version, len(scripts))
	}
	for i := version; i < len(scripts); i++ {
		if _, err := tx.ExecContext(ctx, scripts[i]); err != nil {
			return fmt.Errorf("apply migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}

func (s *Store) bootstrap(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM agent_config").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("create agent identity: %w", err)
		}
		now := time.Now().UTC().UnixMicro()
		_, err = tx.ExecContext(ctx, `INSERT INTO agent_config
			(singleton, agent_id, active_model_profile_id, setup_status, config_version, created_at, updated_at)
			VALUES (1, ?, NULL, 'setup_required', 1, ?, ?)`, id.String(), now, now)
		if err != nil {
			return fmt.Errorf("initialize agent configuration: %w", err)
		}
		e, err := event.New("/erii/agents/"+id.String(), "ai.erii.agent.initialized.v1", id.String(), map[string]any{"agent_id": id.String(), "config_version": 1})
		if err != nil {
			return err
		}
		if err := eventsqlite.Enqueue(ctx, tx, e); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Diagnostics(ctx context.Context) (agent.Diagnostics, error) {
	d := agent.Diagnostics{SchemaVersion: len(migrations), StateDatabase: s.path}
	err := s.db.QueryRowContext(ctx, `SELECT agent_id, active_model_profile_id, setup_status, config_version
		FROM agent_config WHERE singleton = 1`).Scan(&d.AgentID, &d.ActiveModelProfileID, &d.Readiness, &d.ConfigVersion)
	if err != nil {
		return agent.Diagnostics{}, fmt.Errorf("read agent configuration: %w", err)
	}
	if d.ActiveModelProfileID != nil {
		p, err := s.Models().Profile(ctx, *d.ActiveModelProfileID)
		if err != nil || p.Status != "ready" || p.VerifiedAt == nil {
			d.Readiness = agent.Degraded
		}
	}
	return d, nil
}

func (s *Store) Close() error {
	if s.artifacts != nil {
		return errors.Join(s.artifacts.Close(), s.db.Close())
	}
	return s.db.Close()
}
func (s *Store) Artifacts() *artifactsqlite.Store { return s.artifacts }

// Events shares the domain state.db connection and its lifecycle.
func (s *Store) Events() *eventsqlite.Store { return eventsqlite.New(s.db) }

// Models shares state.db, including its migration and transaction lifecycle.
func (s *Store) Models() *llmsqlite.Store { return llmsqlite.New(s.db) }

// Transaction is the domain write boundary. Enqueue events using its tx before
// returning. Commit/rollback errors are not automatically retried.
func (s *Store) Transaction(ctx context.Context, write func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := write(tx); err != nil {
		return err
	}
	return tx.Commit()
}
