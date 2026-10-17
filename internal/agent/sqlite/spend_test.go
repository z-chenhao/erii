package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"path/filepath"
	"sync"
	"testing"

	"github.com/z-chenhao/erii/internal/agent"
	"github.com/z-chenhao/erii/internal/llm"
)

func spendPolicy(t *testing.T, boundary int64) agent.SpendPolicy {
	t.Helper()
	hash, err := llm.EndpointHash(modelInput().Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	return agent.SpendPolicy{Accounts: []agent.SpendAccount{{Account: "openai_compatible:" + hash, Currency: "USD", Boundary: boundary, Prices: []agent.ModelPrice{{Model: "fixture-model", Version: "fixture-v1", Input: 1000000, Output: 1000000, CacheRead: 200000, CacheWrite: 1200000}}}}}
}

// Main was accepted without a spend policy; Root creation freezes the explicit
// policy supplied next. No test mutates frozen pricing or provider observations.
func spendRoot(t *testing.T, boundary int64) (*Store, *Runs, agent.Actor, *loopProvider, agent.RunClaim, agent.Run) {
	t.Helper()
	s, r, a, p := childFixture(t)
	p.respond = childPolicy
	receipt := mustSubmit(t, a, "priced-task", "delegate")
	r.ConfigureSpend(spendPolicy(t, boundary))
	c, err := r.Claim(context.Background(), receipt.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err = executeTaskRun(t, a, r, context.Background(), c); err != nil {
		t.Fatal(err)
	}
	c = rootClaim(t, s, r)
	run, err := r.Load(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	return s, r, a, p, c, run
}

func TestSpendMonetaryConversion(t *testing.T) {
	price := agent.ModelPrice{Input: 1000000, Output: 2000000, CacheRead: 100000, CacheWrite: 1500000}
	amount, err := monetaryCost(price, llm.TokenUsage{InputTokens: 3, OutputTokens: 4, CacheReadTokens: 5, CacheWriteTokens: 7})
	if err != nil || amount != 22 {
		t.Fatal(amount, err)
	} // 3+8+0.5+10.5
	if amount, err = monetaryCost(agent.ModelPrice{Input: 1}, llm.TokenUsage{InputTokens: 1}); err != nil || amount != 1 {
		t.Fatal("round up", amount, err)
	}
	if _, err = monetaryCost(price, llm.TokenUsage{OutputTokens: math.MaxInt64}); err == nil {
		t.Fatal("overflow accepted")
	}
	if _, err = monetaryCost(price, llm.TokenUsage{InputTokens: -1}); err == nil {
		t.Fatal("negative usage accepted")
	}
}

func TestSpendProjectionReachesBoundaryExactly(t *testing.T) {
	// 8192 context * 1.2 microcurrency + 1024 output * 1 = 10854.4,
	// rounded upward to 10855. Equality requires authorization.
	for _, boundary := range []int64{10855, 10856} {
		_, r, _, _, c, run := spendRoot(t, boundary)
		g, err := r.Generation(context.Background(), c, run)
		if err != nil || g.Waiting != (boundary == 10855) {
			t.Fatal(boundary, g, err)
		}
	}
}

func TestSpendActualOverrunHoldsFinalResponseWithoutAnotherModelCall(t *testing.T) {
	for _, cause := range []error{nil, llm.ModelNotFound} {
		for _, decision := range []string{"approved", "denied"} {
			name := decision
			if cause != nil {
				name += "_provider_error"
			}
			t.Run(name, func(t *testing.T) {
				s, r, a, p, c, run := spendRoot(t, 12000)
				ctx := context.Background()
				p.respond = func(req llm.GenerationRequest) (llm.GenerationResult, error) {
					return llm.GenerationResult{Text: "observed final answer", Usage: &llm.TokenUsage{InputTokens: 15000}}, cause
				}
				if err := executeTaskRun(t, a, r, ctx, c); err != nil {
					t.Fatal(err)
				}
				if scalar(t, s, `SELECT count(*) FROM spend_approval WHERE root_run_id='`+run.ID+`' AND status='pending'`) != 1 || scalar(t, s, `SELECT count(*) FROM run WHERE run_id='`+run.ID+`' AND status='waiting'`) != 1 {
					t.Fatal("actual cost crossed boundary but final response escaped consent")
				}
				if scalar(t, s, `SELECT actual_micros FROM spend_reservation WHERE generation_id IN (SELECT generation_id FROM model_generation WHERE run_id='`+run.ID+`')`) != 15000 {
					t.Fatal("actual overrun not recorded")
				}
				v, err := r.Approvals(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if err = r.Decide(ctx, run.AgentID, v[0].ID, v[0].Hash, decision); err != nil {
					t.Fatal(err)
				}
				mustDrain(t, a)
				expected := "completed"
				if cause != nil || decision == "denied" {
					expected = "failed"
				}
				if scalar(t, s, `SELECT count(*) FROM model_generation WHERE run_id='`+run.ID+`'`) != 1 || scalar(t, s, `SELECT count(*) FROM run WHERE run_id='`+run.ID+`' AND status='`+expected+`'`) != 1 {
					t.Fatal("resume regenerated final answer")
				}
			})
		}
	}
}

func TestSpendDenialDrainsAcceptedChildrenAndInvocations(t *testing.T) {
	s, r, a, p, c, run := spendRoot(t, 12000)
	ctx := context.Background()
	p.respond = func(req llm.GenerationRequest) (llm.GenerationResult, error) {
		if req.RunID == run.ID {
			response := calls(agent.RunToolName, `{"task":"one"}`, `{"task":"two"}`, `{"task":"three"}`)
			response.Usage = &llm.TokenUsage{InputTokens: 1000}
			return response, nil
		}
		return llm.GenerationResult{Text: "Child final", Usage: &llm.TokenUsage{InputTokens: 15000}}, nil
	}
	if err := executeTaskRun(t, a, r, ctx, c); err != nil {
		t.Fatal(err)
	}
	mustDrain(t, a)
	v, err := r.Approvals(ctx)
	if err != nil || len(v) != 1 {
		t.Fatal(v, err)
	}
	if err = r.Decide(ctx, run.AgentID, v[0].ID, v[0].Hash, "denied"); err != nil {
		t.Fatal(err)
	}
	mustDrain(t, a)
	if scalar(t, s, `SELECT count(*) FROM run WHERE kind='worker' AND status IN ('queued','running','waiting')`) != 0 || scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE tool_name='builtin__agent__run' AND status='failed'`) != 3 {
		t.Fatal("denial orphaned accepted Child invocations")
	}
	if scalar(t, s, `SELECT count(*) FROM provider_usage u JOIN run r USING(run_id) WHERE r.kind='worker'`) != 2 {
		t.Fatal("denial dispatched extra models")
	}
}

func TestSpendDenialFinalizesUndispatchedGeneration(t *testing.T) {
	s, r, a, _, c, run := spendRoot(t, 1)
	ctx := context.Background()
	if err := executeTaskRun(t, a, r, ctx, c); err != nil {
		t.Fatal(err)
	}
	v, err := r.Approvals(ctx)
	if err != nil || len(v) != 1 {
		t.Fatal(v, err)
	}
	if err = r.Decide(ctx, run.AgentID, v[0].ID, v[0].Hash, "denied"); err != nil {
		t.Fatal(err)
	}
	mustDrain(t, a)
	if scalar(t, s, `SELECT count(*) FROM model_generation WHERE run_id='`+run.ID+`' AND status='failed' AND error_code='SPEND_AUTHORIZATION_DENIED'`) != 1 {
		t.Fatal("denied undispatched model call stayed pending")
	}
	if scalar(t, s, `SELECT count(*) FROM provider_usage WHERE run_id='`+run.ID+`'`) != 0 {
		t.Fatal("denial fabricated usage")
	}
}

func TestSpendProjectedBoundaryRecoveryAndDecisionFences(t *testing.T) {
	for _, decision := range []string{"approved", "denied"} {
		t.Run(decision, func(t *testing.T) {
			s, r, _, _, c, run := spendRoot(t, 1)
			ctx := context.Background()
			gen, err := r.Generation(ctx, c, run)
			if err != nil || !gen.Waiting {
				t.Fatal(gen, err)
			}
			if scalar(t, s, `SELECT count(*) FROM spend_reservation`) != 0 || scalar(t, s, `SELECT count(*) FROM provider_usage u JOIN run r USING(run_id) WHERE r.kind='worker'`) != 0 {
				t.Fatal("held call dispatched/reserved")
			}
			if err = r.Wait(ctx, c, run.Version, "resource"); err != nil {
				t.Fatal(err)
			}
			if _, err = r.Next(ctx); !errors.Is(err, agent.ErrNotFound) {
				t.Fatal("approval waiter spun", err)
			}
			s2, err := Open(ctx, filepath.Dir(s.path))
			if err != nil {
				t.Fatal(err)
			}
			defer s2.Close()
			r2 := s2.Runs(r.history, r.registry) // no live policy; snapshot survives
			views, err := r2.Approvals(ctx)
			if err != nil || len(views) != 1 || views[0].Tool != "account_spend" {
				t.Fatal(views, err)
			}
			v := views[0]
			for _, statement := range []string{`INSERT OR REPLACE INTO spend_approval SELECT * FROM spend_approval`, `UPDATE spend_approval SET arguments_json='{}'`, `DELETE FROM spend_approval`} {
				if _, err = s2.db.Exec(statement); err == nil {
					t.Fatal("frozen monetary approval mutated", statement)
				}
			}
			if err = r2.Decide(ctx, run.AgentID, v.ID, "wrong", decision); !errors.Is(err, agent.ErrConflict) {
				t.Fatal("changed hash accepted", err)
			}
			if err = r2.Decide(ctx, "other", v.ID, v.Hash, decision); !errors.Is(err, agent.ErrConflict) {
				t.Fatal("foreign principal accepted", err)
			}
			if err = r2.Decide(ctx, run.AgentID, v.ID, v.Hash, decision); err != nil {
				t.Fatal(err)
			}
			if err = r2.Decide(ctx, run.AgentID, v.ID, v.Hash, decision); err != nil {
				t.Fatal("duplicate", err)
			}
			opposite := map[string]string{"approved": "denied", "denied": "approved"}[decision]
			if err = r2.Decide(ctx, run.AgentID, v.ID, v.Hash, opposite); !errors.Is(err, agent.ErrConflict) {
				t.Fatal("conflicting decision", err)
			}
			c2, err := r2.Claim(ctx, run.ID)
			if err != nil {
				t.Fatal(err)
			}
			run2, err := r2.Load(ctx, c2)
			if err != nil {
				t.Fatal(err)
			}
			resumed, err := r2.Generation(ctx, c2, run2)
			if err != nil || resumed.Waiting || resumed.Denied != (decision == "denied") || resumed.Request.GenerationID != gen.Request.GenerationID {
				t.Fatal("recovery changed admission", resumed, err)
			}
			if decision == "approved" {
				if err = s2.Models().RecordUsageAnd(ctx, resumed.Request, llm.GenerationResult{Text: "done", Usage: &llm.TokenUsage{InputTokens: 3, OutputTokens: 4}}, 0, nil, s2.AcceptGeneration); err != nil {
					t.Fatal(err)
				}
				if scalar(t, s2, `SELECT actual_micros FROM spend_reservation`) != 7 {
					t.Fatal("actual currency settlement")
				}
				if err = s2.Models().RecordUsageAnd(ctx, resumed.Request, llm.GenerationResult{}, 0, nil, s2.AcceptGeneration); err == nil {
					t.Fatal("duplicate usage accepted")
				}
			}
		})
	}
}

func TestSpendChildrenCoalesceAtRootAcrossProcesses(t *testing.T) {
	s, r, _, _, c, run := spendRoot(t, 12000)
	ctx := context.Background()
	gen, err := r.Generation(ctx, c, run)
	if err != nil || gen.Waiting {
		t.Fatal(gen, err)
	}
	response := calls(agent.RunToolName, `{"task":"one"}`, `{"task":"two"}`, `{"task":"three"}`)
	response.Usage = &llm.TokenUsage{InputTokens: 3000}
	if err = s.Models().RecordUsageAnd(ctx, gen.Request, response, 0, nil, s.AcceptGeneration); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ToolStore(c).Accept(ctx, gen.Request.GenerationID, response.ToolCalls, run.Tools); err != nil {
		t.Fatal(err)
	}
	if err = r.Release(ctx, c); err != nil {
		t.Fatal(err)
	}
	rows, err := s.db.Query(`SELECT run_id FROM run WHERE worker_position='child' ORDER BY run_id`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	s2, err := Open(ctx, filepath.Dir(s.path))
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	r2 := s2.Runs(r.history, r.registry)
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner := r
			if i%2 == 1 {
				runner = r2
			}
			claim, err := runner.Claim(ctx, id)
			if err != nil {
				t.Error(err)
				return
			}
			child, err := runner.Load(ctx, claim)
			if err != nil {
				t.Error(err)
				return
			}
			held, err := runner.Generation(ctx, claim, child)
			if err != nil || !held.Waiting {
				t.Error("Child bypassed aggregate approval", held, err)
				return
			}
			if err = runner.Wait(ctx, claim, child.Version, "resource"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if scalar(t, s, `SELECT count(*) FROM spend_approval`) != 1 || scalar(t, s, `SELECT count(*) FROM notification_intent WHERE message_id LIKE 'spend:%'`) != 1 {
		t.Fatal("Child prompt storm")
	}
	if scalar(t, s, `SELECT count(*) FROM spend_approval a JOIN run r ON r.run_id=a.root_run_id WHERE r.worker_position='root'`) != 1 {
		t.Fatal("not Root owned")
	}
	if scalar(t, s, `SELECT count(*) FROM spend_reservation`) != 1 {
		t.Fatal("blocked Children reserved spend")
	}
	views, err := r2.Approvals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = r2.Decide(ctx, run.AgentID, views[0].ID, views[0].Hash, "approved"); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		claim, err := r.Claim(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		child, err := r.Load(ctx, claim)
		if err != nil {
			t.Fatal(err)
		}
		admitted, err := r.Generation(ctx, claim, child)
		if err != nil || admitted.Waiting || admitted.Denied {
			t.Fatal(admitted, err)
		}
		r.Release(ctx, claim)
	}
	if scalar(t, s, `SELECT count(*) FROM spend_reservation`) != 4 {
		t.Fatal("reservation admission not once per generation")
	}
}

func TestSpendUnpricedUnconfiguredAndMissingUsage(t *testing.T) {
	for _, mode := range []string{"unpriced", "unconfigured", "usage_missing"} {
		t.Run(mode, func(t *testing.T) {
			s, r, a, p := childFixture(t)
			ctx := context.Background()
			p.respond = childPolicy
			receipt := mustSubmit(t, a, "no-guesses", "delegate")
			policy := spendPolicy(t, 100000000)
			if mode == "unpriced" {
				policy.Accounts[0].Prices = nil
			}
			if mode == "unconfigured" {
				policy.Accounts[0].Boundary = 0
			}
			r.ConfigureSpend(policy)
			main, err := r.Claim(ctx, receipt.RunID)
			if err != nil {
				t.Fatal(err)
			}
			if err = executeTaskRun(t, a, r, ctx, main); err != nil {
				t.Fatal(err)
			}
			c := rootClaim(t, s, r)
			run, err := r.Load(ctx, c)
			if err != nil {
				t.Fatal(err)
			}
			r.ConfigureSpend(agent.SpendPolicy{}) // frozen price/policy survives edit
			g, err := r.Generation(ctx, c, run)
			if err != nil || g.Waiting {
				t.Fatal(g, err)
			}
			if err = s.Models().RecordUsageAnd(ctx, g.Request, llm.GenerationResult{Text: "no usage"}, 0, nil, s.AcceptGeneration); err != nil {
				t.Fatal(err)
			}
			if scalar(t, s, `SELECT count(*) FROM spend_approval`) != 0 {
				t.Fatal("invented authorization")
			}
			views, err := r.Spend(ctx)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, v := range views {
				if v.Currency == "USD" {
					found = true
					if mode == "usage_missing" {
						if v.MissingUsage != 1 || v.Committed <= 0 {
							t.Fatal("missing usage became free", v)
						}
					} else if v.State != mode {
						t.Fatal(v)
					}
				}
			}
			if !found {
				t.Fatal(views)
			}
		})
	}
}

func TestSpendSettlementFailureRollsBackUsageAndRecovers(t *testing.T) {
	s, r, _, _, c, run := spendRoot(t, 100000000)
	ctx := context.Background()
	g, err := r.Generation(ctx, c, run)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`CREATE TRIGGER fail_spend BEFORE UPDATE ON spend_reservation BEGIN SELECT RAISE(ABORT,'sensitive provider data'); END`); err != nil {
		t.Fatal(err)
	}
	result := llm.GenerationResult{Text: "ok", Usage: &llm.TokenUsage{InputTokens: 5}}
	if err = s.Models().RecordUsageAnd(ctx, g.Request, result, 0, nil, s.AcceptGeneration); err != agent.ErrStorage {
		t.Fatal("unsanitized or swallowed failure", err)
	}
	if scalar(t, s, `SELECT count(*) FROM provider_usage WHERE run_id='`+run.ID+`'`) != 0 || scalar(t, s, `SELECT count(*) FROM model_generation WHERE run_id='`+run.ID+`' AND status='completed'`) != 0 {
		t.Fatal("partial settlement")
	}
	if _, err = s.db.Exec(`DROP TRIGGER fail_spend`); err != nil {
		t.Fatal(err)
	}
	if err = s.Models().RecordUsageAnd(ctx, g.Request, result, 0, nil, s.AcceptGeneration); err != nil {
		t.Fatal(err)
	}
	if scalar(t, s, `SELECT actual_micros FROM spend_reservation`) != 5 {
		t.Fatal("recovery accounting")
	}
}

func TestSpendMigrationFrom14AndRollback(t *testing.T) {
	s, _, _, _, _, _ := spendRoot(t, 1)
	usage := scalar(t, s, `SELECT count(*) FROM provider_usage`)
	snapshots := scalar(t, s, `SELECT count(*) FROM model_snapshot`)
	for _, broken := range []bool{false, true} {
		db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "state.db"))
		if err != nil {
			t.Fatal(err)
		}
		if err = migrate(context.Background(), db, migrations[:14]); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`ATTACH DATABASE ? AS prior`, s.path); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO model_profile SELECT profile_id,name,provider_kind,endpoint,wire_api,model_id,auth_connection_id,reasoning_effort,context_window,max_output_tokens,generation_defaults_json,compatibility_json,capabilities_json,status,verified_at,created_at,updated_at,1 FROM prior.model_profile`); err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"model_snapshot", "provider_usage"} {
			if _, err = db.Exec(`INSERT INTO ` + table + ` SELECT * FROM prior.` + table); err != nil {
				t.Fatal(table, err)
			}
		}
		next := append([]string{}, migrations...)
		if broken {
			next[14] += "; INVALID DDL"
		}
		err = migrate(context.Background(), db, next)
		var version, tables int
		db.QueryRow(`PRAGMA user_version`).Scan(&version)
		db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name LIKE 'spend_%' AND type='table'`).Scan(&tables)
		if broken {
			if err == nil || version != 14 || tables != 0 {
				t.Fatal("partial migration", version, tables, err)
			}
		} else if err != nil || version != 17 || tables != 3 {
			t.Fatal(version, tables, err)
		}
		if !broken {
			var unpriced int
			if err = db.QueryRow(`SELECT count(*) FROM spend_snapshot WHERE currency='' AND boundary_micros=0 AND price_json='null'`).Scan(&unpriced); err != nil || unpriced != snapshots {
				t.Fatal("historical prices invented", unpriced, err)
			}
		}
		var retained int
		if err = db.QueryRow(`SELECT count(*) FROM provider_usage`).Scan(&retained); err != nil || retained != usage {
			t.Fatal("historical usage lost", retained, err)
		}
		db.Close()
	}
}

func TestSpendCancelledEpochRejectsApprovalAndLateUsage(t *testing.T) {
	s, r, _, _, c, run := spendRoot(t, 1)
	ctx := context.Background()
	g, err := r.Generation(ctx, c, run)
	if err != nil || !g.Waiting {
		t.Fatal(g, err)
	}
	v, err := r.Approvals(ctx)
	if err != nil || len(v) != 1 {
		t.Fatal(v, err)
	}
	if err = r.Wait(ctx, c, run.Version, "resource"); err != nil {
		t.Fatal(err)
	}
	err = r.write(ctx, func(tx *sql.Tx) error {
		var id string
		var epoch, version int64
		if err := tx.QueryRow(`SELECT task_id,task_epoch,version FROM task`).Scan(&id, &epoch, &version); err != nil {
			return err
		}
		return r.stopTask(ctx, tx, id, "cancelled", epoch, version)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Decide(ctx, run.AgentID, v[0].ID, v[0].Hash, "approved"); !errors.Is(err, agent.ErrConflict) {
		t.Fatal("old epoch decision", err)
	}
	if err = s.Models().RecordUsageAnd(ctx, g.Request, llm.GenerationResult{Usage: &llm.TokenUsage{InputTokens: 3}}, 0, nil, s.AcceptGeneration); !errors.Is(err, agent.ErrClaimLost) {
		t.Fatal("late usage", err)
	}
	if scalar(t, s, `SELECT count(*) FROM spend_approval WHERE status='invalidated'`) != 1 || scalar(t, s, `SELECT count(*) FROM run WHERE kind='worker' AND status='cancelled'`) != 1 {
		t.Fatal("stop did not settle")
	}
}

func TestSpendHeldGenerationRejectsUnreservedAcceptance(t *testing.T) {
	s, r, _, _, c, run := spendRoot(t, 1)
	ctx := context.Background()
	g, err := r.Generation(ctx, c, run)
	if err != nil || !g.Waiting {
		t.Fatal(g, err)
	}
	if err = s.Models().RecordUsageAnd(ctx, g.Request, llm.GenerationResult{Text: "unauthorized model output", Usage: &llm.TokenUsage{InputTokens: 100}}, 0, nil, s.AcceptGeneration); err != agent.ErrStorage {
		t.Fatal("held generation accepted", err)
	}
	if scalar(t, s, `SELECT count(*) FROM provider_usage WHERE run_id='`+run.ID+`'`) != 0 || scalar(t, s, `SELECT count(*) FROM spend_reservation`) != 0 || scalar(t, s, `SELECT count(*) FROM spend_approval WHERE status='pending'`) != 1 {
		t.Fatal("unreserved result changed authorization/accounting")
	}
}

func TestSpendMainSteeringKeepsFrozenBatchConsent(t *testing.T) {
	_, r, a, p := childFixture(t)
	ctx := context.Background()
	p.respond = childPolicy
	r.ConfigureSpend(spendPolicy(t, 1))
	receipt := mustSubmit(t, a, "pending-main", "delegate")
	c, err := r.Claim(ctx, receipt.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err = executeTaskRun(t, a, r, ctx, c); err != nil {
		t.Fatal(err)
	}
	before, err := r.Approvals(ctx)
	if err != nil || len(before) != 1 || before[0].Status != "pending" {
		t.Fatal(before, err)
	}
	next := mustSubmit(t, a, "main-steer", "revised user instruction")
	if next.RunID != receipt.RunID {
		t.Fatal("steering changed active Main identity")
	}
	after, err := r.Approvals(ctx)
	if err != nil || len(after) != 1 || after[0] != before[0] {
		t.Fatal("steering invalidated unchanged monetary batch scope", after, err)
	}
}

func TestSpendAccumulatesAcrossRootsAndFreezesPriceVersions(t *testing.T) {
	s, r, a, p, c, run := spendRoot(t, 12000)
	ctx := context.Background()
	g, err := r.Generation(ctx, c, run)
	if err != nil || g.Waiting {
		t.Fatal(g, err)
	}
	if err = s.Models().RecordUsageAnd(ctx, g.Request, llm.GenerationResult{Text: "first", Usage: &llm.TokenUsage{InputTokens: 2000}}, 0, nil, s.AcceptGeneration); err != nil {
		t.Fatal(err)
	}
	if err = r.Release(ctx, c); err != nil {
		t.Fatal(err)
	}
	r.ConfigureSpend(agent.SpendPolicy{})
	receipt := mustSubmit(t, a, "second-priced-task", "delegate again")
	r.ConfigureSpend(spendPolicy(t, 12000))
	p.respond = childPolicy
	main, err := r.Claim(ctx, receipt.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err = executeTaskRun(t, a, r, ctx, main); err != nil {
		t.Fatal(err)
	}
	var next string
	if err = s.db.QueryRow(`SELECT run_id FROM run WHERE worker_position='root' AND run_id!=?`, run.ID).Scan(&next); err != nil {
		t.Fatal(err)
	}
	claim, err := r.Claim(ctx, next)
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.Load(ctx, claim)
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := r.Generation(ctx, claim, second)
	if err != nil || !blocked.Waiting {
		t.Fatal("new Root forgot account spend", blocked, err)
	}
	if scalar(t, s, `SELECT count(*) FROM spend_approval WHERE root_run_id='`+next+`'`) != 1 {
		t.Fatal("wrong grant scope")
	}
	for _, mode := range []string{"currency", "same-version-rates"} {
		policy := spendPolicy(t, 12000)
		if mode == "currency" {
			policy.Accounts[0].Currency = "EUR"
		} else {
			policy.Accounts[0].Prices[0].Input++
		}
		r.ConfigureSpend(policy)
		before := scalar(t, s, `SELECT count(*) FROM model_snapshot`)
		err = s.Transaction(ctx, func(tx *sql.Tx) error {
			frozen, err := s.Models().FreezeProfile(ctx, tx, run.Snapshot.ProfileID, run.Snapshot.ProfileVersion)
			if err != nil {
				return err
			}
			return r.freezeSpend(ctx, tx, frozen)
		})
		if !errors.Is(err, agent.ErrConflict) || scalar(t, s, `SELECT count(*) FROM model_snapshot`) != before {
			t.Fatal("pricing drift did not roll back", mode, err)
		}
	}
	var version string
	if err = s.db.QueryRow(`SELECT json_extract(price_json,'$.version') FROM spend_snapshot WHERE snapshot_id=?`, run.Snapshot.SnapshotID).Scan(&version); err != nil || version != "fixture-v1" {
		t.Fatal("frozen price drift", version, err)
	}
}
