package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/z-chenhao/erii/internal/agent"
	"github.com/z-chenhao/erii/internal/llm"
	"github.com/z-chenhao/erii/internal/task"
	"github.com/z-chenhao/erii/internal/tool"
)

func TestSchedulerDefaultChildCapacity(t *testing.T) {
	s, r, a, p := childFixture(t)
	args := make([]string, 20)
	for i := range args {
		args[i] = fmt.Sprintf(`{"task":"item %d"}`, i)
	}
	c, run, gen, calls := childAdmission(t, s, r, a, p, args...)
	ctx := context.Background()
	if _, err := r.ToolStore(c).Accept(ctx, gen.Request.GenerationID, calls, run.Tools); err != nil {
		t.Fatal(err)
	}
	if err := r.Release(ctx, c); err != nil {
		t.Fatal(err)
	}
	rows, err := s.db.Query(`SELECT run_id FROM run WHERE worker_position='child' ORDER BY created_at,run_id`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	for i, id := range ids[:16] {
		if _, err := r.Claim(ctx, id); err != nil {
			t.Fatalf("default Child slot %d: %v", i+1, err)
		}
	}
	if _, err := r.Claim(ctx, ids[16]); !errors.Is(err, agent.ErrBusy) {
		t.Fatal("17th Child did not queue", err)
	}
	if got := scalar(t, s, `SELECT count(*) FROM run WHERE worker_position='child' AND status='queued'`); got != 4 {
		t.Fatal("capacity rejection changed queued work", got)
	}
}

func schedulerChildren(t *testing.T, r *Runs) []string {
	t.Helper()
	rows, err := r.host.db.Query(`SELECT run_id FROM run WHERE worker_position='child' ORDER BY created_at,run_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return ids
}

func TestSchedulerAtomicQuotasAcrossProcessesAndExpiredOwner(t *testing.T) {
	for _, dimension := range []string{"global", "user", "agent", "provider", "children"} {
		t.Run(dimension, func(t *testing.T) {
			s, r, a, p := childFixture(t)
			l := agent.DefaultSchedulerLimits()
			switch dimension {
			case "global":
				l.Global = 1
			case "user":
				l.User = 1
			case "agent":
				l.Agent = 1
			case "provider":
				l.Provider = 1
			case "children":
				l.Children = 1
			}
			r.ConfigureScheduler(l)
			ctx := context.Background()
			c, run, gen, calls := childAdmission(t, s, r, a, p, `{"task":"one"}`, `{"task":"two"}`)
			if _, err := r.ToolStore(c).Accept(ctx, gen.Request.GenerationID, calls, run.Tools); err != nil {
				t.Fatal(err)
			}
			if err := r.Release(ctx, c); err != nil {
				t.Fatal(err)
			}
			other, err := Open(ctx, filepath.Dir(s.path))
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			r2 := other.Runs(r.history, r.registry)
			r2.ConfigureScheduler(l)
			ids := schedulerChildren(t, r)
			type outcome struct {
				claim agent.RunClaim
				err   error
				index int
			}
			results := make(chan outcome, 2)
			start := make(chan struct{})
			go func() { <-start; c, e := r.Claim(ctx, ids[0]); results <- outcome{c, e, 0} }()
			go func() { <-start; c, e := r2.Claim(ctx, ids[1]); results <- outcome{c, e, 1} }()
			close(start)
			first, second := <-results, <-results
			if first.err != nil {
				first, second = second, first
			}
			if first.err != nil || !errors.Is(second.err, agent.ErrBusy) {
				t.Fatal("simultaneous capacity escaped", first.err, second.err)
			}
			old := first.claim
			if _, err := s.db.Exec(`UPDATE run_claim SET expires_at=0,version=version+1 WHERE run_id=?`, ids[first.index]); err != nil {
				t.Fatal(err)
			}
			if _, err := r2.Claim(ctx, ids[second.index]); err != nil {
				t.Fatal("expired owner retained slot", err)
			}
			if err := r.Renew(ctx, old); !errors.Is(err, agent.ErrClaimLost) {
				t.Fatal("old owner revived", err)
			}
		})
	}
}

func TestSchedulerFairSecondTaskAndDurableTicket(t *testing.T) {
	s, r, a, p := childFixture(t)
	ctx := context.Background()
	c, run, gen, cs := childAdmission(t, s, r, a, p, `{"task":"one"}`, `{"task":"two"}`)
	if _, err := r.ToolStore(c).Accept(ctx, gen.Request.GenerationID, cs, run.Tools); err != nil {
		t.Fatal(err)
	}
	if err := r.Release(ctx, c); err != nil {
		t.Fatal(err)
	}
	id, err := r.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	first, err := r.Claim(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Release(ctx, first); err != nil {
		t.Fatal(err)
	}
	// A second Task arrives after the large tree has already been served.
	receipt := mustSubmit(t, a, "second", "independent work")
	main, err := r.Claim(ctx, receipt.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := executeTaskRun(t, a, r, ctx, main); err != nil {
		t.Fatal(err)
	}
	var second string
	if err := s.db.QueryRow(`SELECT run_id FROM run WHERE worker_position='root' AND run_id!=?`, c.RunID()).Scan(&second); err != nil {
		t.Fatal(err)
	}
	other, err := Open(ctx, filepath.Dir(s.path))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	r2 := other.Runs(r.history, r.registry)
	// A new tree enters at the current queue tail. The older tree may get one
	// turn before it, but cannot drain its backlog first.
	id, err = r2.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if id != second {
		turn, err := r2.Claim(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if err := r2.Release(ctx, turn); err != nil {
			t.Fatal(err)
		}
		id, err = r2.Next(ctx)
	}
	if err != nil || id != second {
		t.Fatal("new Task starved after restart", id, err)
	}
	secondClaim, err := r2.Claim(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if err := r2.Release(ctx, secondClaim); err != nil {
		t.Fatal(err)
	}
	if id, err := r.Next(ctx); err != nil || id == second {
		t.Fatal("queue failed to rotate back", id, err)
	}
}

func TestSchedulerFrozenWorkerTurns(t *testing.T) {
	s, r, a, p := childFixture(t)
	l := agent.DefaultSchedulerLimits()
	l.Turns = 2
	r.ConfigureScheduler(l)
	p.respond = func(req llm.GenerationRequest) (llm.GenerationResult, error) {
		if isFollowup(req) || (req.Messages[len(req.Messages)-1].Role == "tool" && !isRoot(req)) {
			return llm.GenerationResult{Text: "Main receipt"}, nil
		}
		if !isRoot(req) {
			return calls(task.ManageName, `{"operation":"create","task":"loop"}`), nil
		}
		return calls("builtin__text__echo", `{"text":"loop"}`), nil
	}
	mustSubmit(t, a, "loop", "delegate")
	mustDrain(t, a)
	if scalar(t, s, `SELECT count(*) FROM model_generation g JOIN run r USING(run_id) WHERE r.kind='worker'`) != 2 || scalar(t, s, `SELECT count(*) FROM run WHERE error_code='RESOURCE_LIMIT'`) != 1 {
		t.Fatal("turn ceiling failed")
	}
	if scalar(t, s, `SELECT max_turns FROM run_tree`) != 2 {
		t.Fatal("turn ceiling not frozen")
	}
	if _, err := s.db.Exec(`UPDATE run_tree SET max_turns=3,version=version+1`); err == nil {
		t.Fatal("turn ceiling mutable")
	}
}

func TestSchedulerInfrastructureCircuitAndBusinessFailureIsolation(t *testing.T) {
	s, r, a, p := childFixture(t)
	ctx := context.Background()
	args := make([]string, 9)
	for i := range args {
		args[i] = fmt.Sprintf(`{"task":"item %d"}`, i)
	}
	c, run, gen, cs := childAdmission(t, s, r, a, p, args...)
	if _, err := r.ToolStore(c).Accept(ctx, gen.Request.GenerationID, cs, run.Tools); err != nil {
		t.Fatal(err)
	}
	if err := r.Release(ctx, c); err != nil {
		t.Fatal(err)
	}
	ids := schedulerChildren(t, r)
	accept := func(i int, cause error) {
		t.Helper()
		claim, err := r.Claim(ctx, ids[i])
		if err != nil {
			t.Fatal(err)
		}
		child, err := r.Load(ctx, claim)
		if err != nil {
			t.Fatal(err)
		}
		generation, err := r.Generation(ctx, claim, child)
		if err != nil {
			t.Fatal(err)
		}
		result := llm.GenerationResult{Text: "item result", Usage: &llm.TokenUsage{InputTokens: 10, OutputTokens: 3}}
		if err := s.Models().RecordUsageAnd(ctx, generation.Request, result, time.Millisecond, cause, s.AcceptGeneration); err != nil {
			t.Fatal(err)
		}
		if err := r.Release(ctx, claim); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		accept(i, llm.ModelNotFound)
	}
	if scalar(t, s, `SELECT count(*) FROM scheduler_provider WHERE blocked_until>0`) != 0 {
		t.Fatal("business failures opened circuit")
	}
	for i := 3; i < 8; i++ {
		accept(i, llm.ProviderUnavailable)
	}
	if _, err := r.Claim(ctx, ids[8]); !errors.Is(err, agent.ErrBusy) {
		t.Fatal("open circuit admitted next item", err)
	}
	if _, err := r.Next(ctx); !errors.Is(err, agent.ErrBusy) {
		t.Fatal("cooldown lost queued work", err)
	}
	if scalar(t, s, `SELECT consecutive_errors FROM scheduler_provider`) != 5 || scalar(t, s, `SELECT count(*) FROM provider_usage WHERE error_code='PROVIDER_UNAVAILABLE'`) != 5 {
		t.Fatal("breaker and usage not atomic")
	}
	if _, err := s.db.Exec(`UPDATE scheduler_provider SET blocked_until=?`, time.Now().Add(-time.Second).UnixMicro()); err != nil {
		t.Fatal(err)
	}
	accept(8, nil)
	if scalar(t, s, `SELECT consecutive_errors FROM scheduler_provider`) != 0 {
		t.Fatal("success did not reset circuit")
	}
}

func TestSchedulerActivityCountsDoNotNotify(t *testing.T) {
	s, r, a, p := childFixture(t)
	c, run, gen, cs := childAdmission(t, s, r, a, p, `{"task":"one"}`, `{"task":"two"}`)
	ctx := context.Background()
	if _, err := r.ToolStore(c).Accept(ctx, gen.Request.GenerationID, cs, run.Tools); err != nil {
		t.Fatal(err)
	}
	before := scalar(t, s, `SELECT count(*) FROM event_outbox`)
	items, err := r.Activity(ctx)
	if err != nil || len(items) != 1 || !strings.Contains(items[0].ProgressSummary, "queued 2, running 0, succeeded 0, failed 0, waiting 0") {
		t.Fatal(items, err)
	}
	if scalar(t, s, `SELECT count(*) FROM event_outbox`) != before || scalar(t, s, `SELECT count(*) FROM task_update`) != 0 {
		t.Fatal("progress woke Main")
	}
}

func TestSchedulerDrainWaitsForStoppedLiveOwner(t *testing.T) {
	s, r, a, p := childFixture(t)
	ctx := context.Background()
	c, _, _, _ := childAdmission(t, s, r, a, p)
	if err := r.write(ctx, func(tx *sql.Tx) error {
		var id string
		var epoch, version int64
		if err := tx.QueryRowContext(ctx, `SELECT t.task_id,t.task_epoch,t.version FROM task t JOIN run r USING(task_id) WHERE r.run_id=?`, c.RunID()).Scan(&id, &epoch, &version); err != nil {
			return err
		}
		return r.stopTask(ctx, tx, id, "cancelled", epoch, version)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Next(ctx); !errors.Is(err, agent.ErrBusy) {
		t.Fatal("drain finished before cancelled live owner joined/expired", err)
	}
	if _, err := s.db.Exec(`UPDATE run_claim SET expires_at=0,version=version+1 WHERE run_id=?`, c.RunID()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Next(ctx); !errors.Is(err, agent.ErrNotFound) {
		t.Fatal("expired stop did not settle", err)
	}
	if scalar(t, s, `SELECT count(*) FROM run WHERE kind='worker' AND status='cancelled'`) != 1 || scalar(t, s, `SELECT count(*) FROM run_claim WHERE token IS NOT NULL`) != 0 {
		t.Fatal("cancelled owner retained work")
	}
}

func TestSchedulerReadyToolProgressesBesidePendingApproval(t *testing.T) {
	echo, _ := tool.EchoRegistry().Resolve("builtin__text__echo")
	cs := append(effectCall(`{"target":"Alex","text":"Tuesday"}`), llm.ToolCall{ID: "retry", Name: echo.Spec.Name, Arguments: `{"text":"independent"}`})
	s, r, c, e, exchange := admitPolicy(t, []tool.Definition{effectDef(t, "query_before_retry"), echo}, cs)
	ctx := context.Background()
	invocations, err := e.Store.Accept(ctx, exchange, cs, e.Registry.Describe())
	if err != nil {
		t.Fatal(err)
	}
	policy := e.Store.(tool.PolicyStore)
	for _, inv := range invocations {
		if _, err := policy.Prepare(ctx, inv.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.Store.Start(ctx, invocations[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := policy.Defer(ctx, invocations[1].ID, "retry_backoff", time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	run, err := r.Load(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Wait(ctx, c, run.Version, "resource"); err != nil {
		t.Fatal(err)
	}
	if id, err := r.Next(ctx); err != nil || id != c.RunID() {
		t.Fatal("pending approval blocked independent ready Tool", id, err)
	}
	resumed, err := r.Claim(ctx, c.RunID())
	if err != nil {
		t.Fatal(err)
	}
	e.Store = r.ToolStore(resumed)
	if _, err := e.Execute(ctx, exchange, cs); !errors.Is(err, tool.ErrSuspended) {
		t.Fatal("pending effect did not remain suspended", err)
	}
	if scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE provider_call_id='retry' AND status='succeeded'`) != 1 || scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE provider_call_id='effect' AND status='waiting' AND attempt_count=0`) != 1 {
		t.Fatal("ready Tool did not progress independently of frozen effect approval")
	}
	if err := r.Release(ctx, resumed); err != nil {
		t.Fatal(err)
	}
	if scalar(t, s, `SELECT count(*) FROM approval WHERE status='pending'`) != 1 {
		t.Fatal("scheduler changed effect approval")
	}
}

func TestSchedulerHealthWriteFailureIsRecoverableAndAtomic(t *testing.T) {
	s, r, a, p := childFixture(t)
	ctx := context.Background()
	c, run, gen, cs := childAdmission(t, s, r, a, p, `{"task":"one"}`)
	if _, err := r.ToolStore(c).Accept(ctx, gen.Request.GenerationID, cs, run.Tools); err != nil {
		t.Fatal(err)
	}
	child, err := r.Claim(ctx, schedulerChildren(t, r)[0])
	if err != nil {
		t.Fatal(err)
	}
	work, err := r.Load(ctx, child)
	if err != nil {
		t.Fatal(err)
	}
	generation, err := r.Generation(ctx, child, work)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_health BEFORE INSERT ON scheduler_provider BEGIN SELECT RAISE(ABORT,'private storage detail'); END`); err != nil {
		t.Fatal(err)
	}
	result := llm.GenerationResult{Text: "result", Usage: &llm.TokenUsage{InputTokens: 10, OutputTokens: 2}}
	err = s.Models().RecordUsageAnd(ctx, generation.Request, result, time.Millisecond, nil, s.AcceptGeneration)
	if !errors.Is(err, agent.ErrStorage) {
		t.Fatal("health persistence failure escaped infrastructure classification", err)
	}
	if scalar(t, s, `SELECT count(*) FROM model_generation WHERE generation_id=? AND status='pending'`, generation.Request.GenerationID) != 1 || scalar(t, s, `SELECT count(*) FROM provider_usage WHERE generation_id=?`, generation.Request.GenerationID) != 0 {
		t.Fatal("health fault partially committed generation/usage")
	}
	if _, err := s.db.Exec(`DROP TRIGGER fail_health`); err != nil {
		t.Fatal(err)
	}
	if err := s.Models().RecordUsageAnd(ctx, generation.Request, result, time.Millisecond, nil, s.AcceptGeneration); err != nil {
		t.Fatal("acceptance did not recover", err)
	}
	if scalar(t, s, `SELECT count(*) FROM provider_usage WHERE generation_id=?`, generation.Request.GenerationID) != 1 {
		t.Fatal("recovery duplicated usage")
	}
}

func TestSchedulerMigrationV13PreservesFrozenTreeAndRollsBack(t *testing.T) {
	s, r, a, p := childFixture(t)
	p.respond = childPolicy
	receipt := mustSubmit(t, a, "migration", "delegate")
	main, err := r.Claim(context.Background(), receipt.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := executeTaskRun(t, a, r, context.Background(), main); err != nil {
		t.Fatal(err)
	}
	var rootID string
	if err := s.db.QueryRow(`SELECT run_id FROM run WHERE worker_position='root'`).Scan(&rootID); err != nil {
		t.Fatal(err)
	}
	for _, broken := range []bool{false, true} {
		t.Run(fmt.Sprint(broken), func(t *testing.T) {
			db, err := sql.Open("sqlite", "file::memory:?_txlock=immediate&_pragma=foreign_keys(1)")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			ctx := context.Background()
			if err := migrate(ctx, db, migrations[:13]); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`ATTACH DATABASE ? AS prior`, s.path); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO model_profile SELECT profile_id,name,provider_kind,endpoint,wire_api,model_id,auth_connection_id,reasoning_effort,context_window,max_output_tokens,generation_defaults_json,compatibility_json,capabilities_json,status,verified_at,created_at,updated_at,1 FROM prior.model_profile`); err != nil {
				t.Fatal(err)
			}
			for _, table := range []string{"model_snapshot", "agent_config", "capability_snapshot", "task"} {
				if _, err := db.Exec(`INSERT INTO ` + table + ` SELECT * FROM prior.` + table); err != nil {
					t.Fatal(table, err)
				}
			}
			if _, err := db.Exec(`INSERT INTO run SELECT * FROM prior.run WHERE worker_position='root'`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO run_tree(root_run_id,descendants,tool_calls,max_depth,max_descendants,max_tool_calls,version) SELECT root_run_id,descendants,tool_calls,max_depth,max_descendants,max_tool_calls,version FROM prior.run_tree`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE run_tree SET tool_calls=tool_calls+1,version=version+1`); err != nil {
				t.Fatal(err)
			}
			next := append([]string{}, migrations...)
			if broken {
				next[13] += "; INVALID SQL"
			}
			err = migrate(ctx, db, next)
			var version, columns int
			if e := db.QueryRow(`PRAGMA user_version`).Scan(&version); e != nil {
				t.Fatal(e)
			}
			if e := db.QueryRow(`SELECT count(*) FROM pragma_table_info('run_tree') WHERE name='max_turns'`).Scan(&columns); e != nil {
				t.Fatal(e)
			}
			if broken {
				if err == nil || version != 13 || columns != 0 {
					t.Fatal("partial scheduler migration", err, version, columns)
				}
				return
			}
			if err != nil || version != 17 || columns != 1 {
				t.Fatal(err, version, columns)
			}
			var identical, turns, calls, ticket int
			if err := db.QueryRow(`SELECT r.checkpoint_json=p.checkpoint_json AND r.model_snapshot_id=p.model_snapshot_id AND r.capability_snapshot_id=p.capability_snapshot_id,t.max_turns,t.tool_calls,q.last_served FROM run r JOIN prior.run p USING(run_id) JOIN run_tree t ON t.root_run_id=r.run_id JOIN scheduler_tree q ON q.root_run_id=r.run_id WHERE r.run_id=?`, rootID).Scan(&identical, &turns, &calls, &ticket); err != nil {
				t.Fatal(err)
			}
			if identical != 1 || turns != 256 || calls != 1 || ticket != 0 {
				t.Fatal("migration rewrote tree", identical, turns, calls, ticket)
			}
			if err := migrate(ctx, db, migrations); err != nil {
				t.Fatal("repeat migration", err)
			}
		})
	}
}
