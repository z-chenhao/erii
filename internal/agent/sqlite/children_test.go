package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/z-chenhao/erii/internal/agent"
	"github.com/z-chenhao/erii/internal/artifact"
	artifacttools "github.com/z-chenhao/erii/internal/artifact/tools"
	"github.com/z-chenhao/erii/internal/llm"
	"github.com/z-chenhao/erii/internal/task"
	"github.com/z-chenhao/erii/internal/tool"
)

func childFixture(t *testing.T, more ...tool.Definition) (*Store, *Runs, agent.Actor, *loopProvider) {
	t.Helper()
	s, r, a, p := taskFixture(t)
	defs := append(artifacttools.Definitions(), agent.AgentToolDefinition())
	defs = append(defs, more...)
	for _, spec := range a.Registry.Describe() {
		d, _ := a.Registry.Resolve(spec.Name)
		defs = append(defs, d)
	}
	reg, err := tool.NewRegistry(defs)
	if err != nil {
		t.Fatal(err)
	}
	r.registry, a.Registry = reg, reg
	return s, r, a, p
}
func childPolicy(req llm.GenerationRequest) (llm.GenerationResult, error) {
	last := req.Messages[len(req.Messages)-1]
	if last.Role == "tool" {
		return llm.GenerationResult{Text: "aggregated: " + last.Text}, nil
	}
	if isFollowup(req) {
		return llm.GenerationResult{Text: "Main final delivery"}, nil
	}
	if !isRoot(req) {
		return calls(task.ManageName, `{"operation":"create","task":"root private instruction"}`), nil
	}
	switch last.Text {
	case "child explicit input":
		return calls(agent.RunToolName, `{"task":"grandchild explicit input"}`), nil
	case "grandchild explicit input":
		return llm.GenerationResult{Text: "grandchild final result"}, nil
	default:
		return calls(agent.RunToolName, `{"task":"child explicit input"}`), nil
	}
}

func TestChildDirectToolRootChildGrandchild(t *testing.T) {
	s, _, a, p := childFixture(t)
	p.respond = childPolicy
	mustSubmit(t, a, "tree", "Main private conversation")
	mustDrain(t, a)
	for query, want := range map[string]int{
		`SELECT count(*) FROM task`:                                                     1,
		`SELECT count(*) FROM task WHERE status='completed'`:                            1,
		`SELECT count(*) FROM run WHERE worker_position='root'`:                         1,
		`SELECT count(*) FROM run WHERE worker_position='child' AND status='completed'`: 2,
		`SELECT count(*) FROM agent_invocation`:                                         2,
		`SELECT descendants FROM run_tree`:                                              2,
		`SELECT tool_calls FROM run_tree`:                                               2,
		`SELECT count(*) FROM task_update`:                                              1,
		`SELECT count(*) FROM delivery_request`:                                         2,
		`SELECT count(*) FROM run c JOIN run p ON c.parent_run_id=p.run_id WHERE c.model_snapshot_id=p.model_snapshot_id AND c.task_id=p.task_id AND c.depth=p.depth+1 AND c.root_run_id=p.root_run_id`: 2,
	} {
		if got := scalar(t, s, query); got != want {
			t.Fatalf("%s = %d, want %d", query, got, want)
		}
	}
	for _, req := range p.requests {
		if !isRoot(req) {
			for _, spec := range req.Tools {
				if spec.Name == agent.RunToolName {
					t.Fatal("Main delegation visible")
				}
			}
			continue
		}
		if strings.Contains(req.Messages[2].Text, "root private instruction") {
			t.Fatal("wrong prompt fragments")
		}
		if req.Messages[1].Text == "You are Erii. Follow stable safety and evidence principles." {
			continue
		}
		raw, _ := json.Marshal(req.Messages)
		if strings.Contains(string(raw), "Main private") || strings.Contains(string(raw), "root private") || strings.Contains(string(raw), "Task anchor") {
			t.Fatalf("Child inherited private context: %s", raw)
		}
	}
	rows, err := s.db.Query(`SELECT result_json FROM tool_invocation WHERE tool_name=?`, agent.RunToolName)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		rows.Scan(&raw)
		var result tool.Result
		json.Unmarshal([]byte(raw), &result)
		var value map[string]any
		if json.Unmarshal([]byte(result.ModelText()), &value) != nil || len(value) != 2 || value["text"] == nil || value["artifactRefs"] == nil {
			t.Fatal("AgentResult leaked Host state", raw)
		}
	}
	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}
}

// Persist the provider exchange so the admission path cannot be bypassed by a
// fabricated call ID or a model-supplied Run identity.
func childAdmission(t *testing.T, s *Store, r *Runs, a agent.Actor, p *loopProvider, args ...string) (agent.RunClaim, agent.Run, agent.Generation, []llm.ToolCall) {
	t.Helper()
	ctx := context.Background()
	p.respond = childPolicy
	receipt := mustSubmit(t, a, "tree", "delegate")
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
	gen, err := r.Generation(ctx, c, run)
	if err != nil {
		t.Fatal(err)
	}
	response := calls(agent.RunToolName, args...)
	if err = s.Models().RecordUsageAnd(ctx, gen.Request, response, 0, nil, s.AcceptGeneration); err != nil {
		t.Fatal(err)
	}
	return c, run, gen, response.ToolCalls
}
func TestChildAtomicAdmissionReplayAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name     string
		limits   agent.ChildLimits
		args     []string
		children int
		code     string
	}{
		{"valid", agent.DefaultChildLimits(), []string{`{"task":"child"}`}, 1, ""},
		{"distinct calls are distinct children", agent.DefaultChildLimits(), []string{`{"task":"child"}`, `{"task":"child"}`}, 2, ""},
		{"descendant ceiling", agent.ChildLimits{Depth: 8, Descendants: 1, ToolCalls: 10000}, []string{`{"task":"child"}`, `{"task":"child"}`}, 1, "RESOURCE_LIMIT"},
		{"call ceiling", agent.ChildLimits{Depth: 8, Descendants: 4096, ToolCalls: 1}, []string{`{"task":"child"}`, `{"task":"child"}`}, 1, "RESOURCE_LIMIT"},
		{"profile expansion", agent.DefaultChildLimits(), []string{`{"task":"child","model":"other"}`}, 0, "INVALID_ARGUMENT"},
		{"artifact expansion", agent.DefaultChildLimits(), []string{`{"task":"child","artifactRefs":["artifact:forged"]}`}, 0, "NOT_FOUND"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, a, p := childFixture(t)
			r.ConfigureChildLimits(tc.limits)
			c, run, gen, calls := childAdmission(t, s, r, a, p, tc.args...)
			ts := r.ToolStore(c)
			invs, err := ts.Accept(context.Background(), gen.Request.GenerationID, calls, run.Tools)
			if err != nil {
				t.Fatal(err)
			}
			if scalar(t, s, `SELECT count(*) FROM agent_invocation`) != tc.children || scalar(t, s, `SELECT descendants FROM run_tree`) != tc.children || scalar(t, s, `SELECT count(*) FROM task`) != 1 {
				t.Fatal("admission counters or Task delta")
			}
			if tc.code != "" && (invs[len(invs)-1].Result.Error == nil || invs[len(invs)-1].Result.Error.Code != tc.code) {
				t.Fatal(invs)
			}
			again, err := ts.Accept(context.Background(), gen.Request.GenerationID, calls, run.Tools)
			if err != nil || !reflect.DeepEqual(invs, again) || scalar(t, s, `SELECT descendants FROM run_tree`) != tc.children {
				t.Fatal("duplicate admission", again, err)
			}
			calls[0].Arguments = `{"task":"changed"}`
			if _, err = ts.Accept(context.Background(), gen.Request.GenerationID, calls, run.Tools); !errors.Is(err, tool.ErrReplay) {
				t.Fatal("parameter drift", err)
			}
		})
	}
}
func TestChildAdmissionRollsBackInvocationCounterAndOutbox(t *testing.T) {
	s, r, a, p := childFixture(t)
	c, run, gen, calls := childAdmission(t, s, r, a, p, `{"task":"child"}`)
	before := scalar(t, s, `SELECT count(*) FROM event_outbox`)
	if _, err := s.db.Exec(`CREATE TRIGGER fail_child BEFORE INSERT ON agent_invocation BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ToolStore(c).Accept(context.Background(), gen.Request.GenerationID, calls, run.Tools); !errors.Is(err, agent.ErrStorage) {
		t.Fatal(err)
	}
	if scalar(t, s, `SELECT count(*) FROM run WHERE worker_position='child'`) != 0 || scalar(t, s, `SELECT descendants FROM run_tree`) != 0 || scalar(t, s, `SELECT tool_calls FROM run_tree`) != 0 || scalar(t, s, `SELECT count(*) FROM event_outbox`) != before || scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE run_id=?`, c.RunID()) != 0 {
		t.Fatal("partial child admission")
	}
}
func TestChildAdmissionConcurrentDuplicateAndOldOwner(t *testing.T) {
	s, r, a, p := childFixture(t)
	c, run, gen, calls := childAdmission(t, s, r, a, p, `{"task":"child"}`)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := r.ToolStore(c).Accept(context.Background(), gen.Request.GenerationID, calls, run.Tools)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if scalar(t, s, `SELECT count(*) FROM agent_invocation`) != 1 || scalar(t, s, `SELECT descendants FROM run_tree`) != 1 {
		t.Fatal("duplicate Child")
	}
	if err := r.Release(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ToolStore(c).Accept(context.Background(), gen.Request.GenerationID, calls, run.Tools); !errors.Is(err, agent.ErrClaimLost) {
		t.Fatal("old owner", err)
	}
}
func TestChildFrozenModelAndCapabilitiesAfterProfileSwitch(t *testing.T) {
	s, r, a, p := childFixture(t)
	c, run, gen, calls := childAdmission(t, s, r, a, p, `{"task":"child"}`)
	input := modelInput()
	input.ModelID = "new-model"
	activateLoop(t, s, input)
	if _, err := r.ToolStore(c).Accept(context.Background(), gen.Request.GenerationID, calls, run.Tools); err != nil {
		t.Fatal(err)
	}
	if scalar(t, s, `SELECT count(*) FROM run c JOIN run p ON p.run_id=c.parent_run_id JOIN capability_snapshot cc ON cc.snapshot_id=c.capability_snapshot_id JOIN capability_snapshot pc ON pc.snapshot_id=p.capability_snapshot_id WHERE c.model_snapshot_id=p.model_snapshot_id AND cc.tools_json=pc.tools_json AND cc.skill_set_hash=pc.skill_set_hash AND cc.data_scope=pc.data_scope AND cc.agent_config_version=pc.agent_config_version`) != 1 {
		t.Fatal("Child used live profile/capabilities")
	}
}
func TestChildDepthCeilingAndBusinessFailureOnlyReachParent(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			s, r, a, p := childFixture(t)
			r.ConfigureChildLimits(agent.ChildLimits{Depth: 1, Descendants: 4096, ToolCalls: 10000})
			p.respond = func(req llm.GenerationRequest) (llm.GenerationResult, error) {
				if failure && req.Messages[len(req.Messages)-1].Text == "child explicit input" {
					return llm.GenerationResult{}, llm.ModelNotFound
				}
				return childPolicy(req)
			}
			mustSubmit(t, a, "one", "delegate")
			mustDrain(t, a)
			if scalar(t, s, `SELECT count(*) FROM run WHERE worker_position='child'`) != 1 || scalar(t, s, `SELECT count(*) FROM task_update`) != 1 {
				t.Fatal("Child messaged Main or exceeded depth")
			}
			code := "RESOURCE_LIMIT"
			if failure {
				code = "PROVIDER_UNAVAILABLE"
			}
			if scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE json_extract(error_json,'$.code')=?`, code) != 1 {
				t.Fatal("business error not returned to Parent", code)
			}
		})
	}
}
func TestChildMigrationV12PreservesFrozenRunsAndAtomicRollback(t *testing.T) {
	for _, broken := range []bool{false, true} {
		t.Run(fmt.Sprint(broken), func(t *testing.T) {
			db, err := sql.Open("sqlite", "file::memory:?_txlock=immediate")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			ctx := context.Background()
			if err = migrate(ctx, db, migrations[:12]); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(`INSERT INTO agent_config VALUES(1,'prior',NULL,'setup_required',7,100,100)`); err != nil {
				t.Fatal(err)
			}
			next := append([]string{}, migrations...)
			if broken {
				next[12] += "; INVALID SQL"
			}
			err = migrate(ctx, db, next)
			var version int
			db.QueryRow(`PRAGMA user_version`).Scan(&version)
			if broken {
				if err == nil || version != 12 {
					t.Fatal("migration partially committed", err, version)
				}
			} else if err != nil || version != 17 {
				t.Fatal(err, version)
			}
			var config int
			db.QueryRow(`SELECT config_version FROM agent_config`).Scan(&config)
			if config != 7 {
				t.Fatal("configuration rewritten")
			}
		})
	}
}

func TestChildApprovalIsPerInvocationAndWaitingTreeReleasesLeases(t *testing.T) {
	s, r, a, p := childFixture(t, effectDef(t, "query_before_retry"))
	p.respond = func(req llm.GenerationRequest) (llm.GenerationResult, error) {
		last := req.Messages[len(req.Messages)-1]
		if last.Role == "tool" || isFollowup(req) {
			return llm.GenerationResult{Text: "done"}, nil
		}
		if !isRoot(req) {
			return calls(task.ManageName, `{"operation":"create","task":"approval tree"}`), nil
		}
		effect := calls("mcp__fixture__send", `{"target":"Alex","text":"Tuesday"}`)
		if strings.Contains(last.Text, "Task anchor:") {
			effect.ToolCalls = append(effect.ToolCalls, llm.ToolCall{ID: "child", Name: agent.RunToolName, Arguments: `{"task":"child effect"}`})
		}
		return effect, nil
	}
	mustSubmit(t, a, "one", "delegate")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.Drain(ctx); err != nil {
		t.Fatal("waiting tree retained live resources", err)
	}
	if scalar(t, s, `SELECT count(*) FROM approval WHERE status='pending'`) != 2 || scalar(t, s, `SELECT count(*) FROM run_claim WHERE token IS NOT NULL`) != 0 {
		t.Fatal("approval inherited or live lease retained")
	}
	var rootApproval, hash string
	if err := s.db.QueryRow(`SELECT a.approval_id,a.frozen_arguments_hash FROM approval a JOIN tool_invocation i USING(invocation_id) JOIN run r USING(run_id) WHERE r.worker_position='root'`).Scan(&rootApproval, &hash); err != nil {
		t.Fatal(err)
	}
	if err := r.Decide(ctx, idAgent(t, s), rootApproval, hash, "approved"); err != nil {
		t.Fatal(err)
	}
	if err := a.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if scalar(t, s, `SELECT count(*) FROM approval WHERE status='pending'`) != 1 || scalar(t, s, `SELECT count(*) FROM tool_invocation i JOIN run r USING(run_id) WHERE r.worker_position='child' AND i.status='waiting' AND i.wait_reason='approval'`) != 1 {
		t.Fatal("Parent approval authorized Child")
	}
	views, err := r.Approvals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range views {
		if v.Status == "pending" {
			if err = r.Decide(ctx, idAgent(t, s), v.ID, v.Hash, "denied"); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = a.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if scalar(t, s, `SELECT count(*) FROM tool_invocation i JOIN run r USING(run_id) WHERE r.worker_position='child' AND i.status='failed' AND json_extract(i.error_json,'$.code')='APPROVAL_DENIED'`) != 1 {
		t.Fatal("Child rejection missing")
	}
	if scalar(t, s, `SELECT count(*) FROM task_update`) != 1 {
		t.Fatal("approval produced per-Child followups")
	}
}

func TestChildRejectsRealPrivateArtifactAndLateEpochResult(t *testing.T) {
	s, r, a, p := childFixture(t)
	// The private Main output exists, but has never been granted to this Task.
	receipt := mustSubmit(t, a, "private", "private")
	private := putArtifact(t, s, artifact.Scope{UserID: artifact.LocalUser, AgentID: idAgent(t, s), RunID: receipt.RunID}, "private Main output")
	p.respond = func(llm.GenerationRequest) (llm.GenerationResult, error) {
		return llm.GenerationResult{Text: "private"}, nil
	}
	mustDrain(t, a)
	args, _ := json.Marshal(agent.AgentRunRequest{Task: "child", ArtifactRefs: []string{private}})
	c, run, gen, calls := childAdmission(t, s, r, a, p, string(args), `{"task":"child"}`)
	ctx := context.Background()
	invs, err := r.ToolStore(c).Accept(ctx, gen.Request.GenerationID, calls, run.Tools)
	if err != nil {
		t.Fatal(err)
	}
	if invs[0].Result.Error == nil || invs[0].Result.Error.Code != "NOT_FOUND" || scalar(t, s, `SELECT count(*) FROM agent_invocation`) != 1 {
		t.Fatal("private artifact expanded", invs)
	}
	var childID string
	if err = s.db.QueryRow(`SELECT child_run_id FROM agent_invocation`).Scan(&childID); err != nil {
		t.Fatal(err)
	}
	child, err := r.Claim(ctx, childID)
	if err != nil {
		t.Fatal(err)
	}
	cr, err := r.Load(ctx, child)
	if err != nil {
		t.Fatal(err)
	}
	cg, err := r.Generation(ctx, child, cr)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.write(ctx, func(tx *sql.Tx) error {
		var taskID string
		var epoch, version int64
		if err := tx.QueryRowContext(ctx, `SELECT t.task_id,t.task_epoch,t.version FROM task t JOIN run r USING(task_id) WHERE r.run_id=?`, childID).Scan(&taskID, &epoch, &version); err != nil {
			return err
		}
		return r.stopTask(ctx, tx, taskID, "cancelled", epoch, version)
	}); err != nil {
		t.Fatal(err)
	}
	if err = s.Models().RecordUsageAnd(ctx, cg.Request, llm.GenerationResult{Text: "late result"}, 0, nil, s.AcceptGeneration); !errors.Is(err, agent.ErrClaimLost) {
		t.Fatal("old Task epoch result accepted", err)
	}
	if _, err = r.ToolStore(child).Accept(ctx, cg.Request.GenerationID, nil, cr.Tools); !errors.Is(err, agent.ErrClaimLost) {
		t.Fatal("old epoch tool call accepted", err)
	}
	if scalar(t, s, `SELECT count(*) FROM task_update`) != 0 {
		t.Fatal("late Child delivered to Main")
	}
}

func TestChildUnknownEffectNeverBecomesSuccessOrNewChild(t *testing.T) {
	for _, modelFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(modelFailure), func(t *testing.T) {
			s, r, a, p := childFixture(t, effectDef(t, "never"))
			p.respond = func(req llm.GenerationRequest) (llm.GenerationResult, error) {
				last := req.Messages[len(req.Messages)-1]
				if last.Role == "tool" && modelFailure && strings.Contains(req.Messages[1].Text, "general Worker executing") {
					return llm.GenerationResult{}, llm.ModelNotFound
				}
				if last.Role == "tool" || isFollowup(req) {
					return llm.GenerationResult{Text: "uncertain effect"}, nil
				}
				if !isRoot(req) {
					return calls(task.ManageName, `{"operation":"create","task":"unknown tree"}`), nil
				}
				if strings.Contains(last.Text, "Task anchor:") {
					return calls(agent.RunToolName, `{"task":"child effect"}`), nil
				}
				return calls("mcp__fixture__send", `{"target":"Alex","text":"Tuesday","behavior":"unknown"}`), nil
			}
			mustSubmit(t, a, "one", "delegate")
			mustDrain(t, a)
			approvePolicy(t, s, r)
			mustDrain(t, a)
			if scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE status='unknown'`) != 2 || scalar(t, s, `SELECT count(*) FROM task WHERE status='active'`) != 1 || scalar(t, s, `SELECT count(*) FROM task_update WHERE kind='unknown'`) != 1 {
				t.Fatal("Child uncertainty became success")
			}
			mustDrain(t, a)
			if scalar(t, s, `SELECT descendants FROM run_tree`) != 1 || scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE tool_name='mcp__fixture__send' AND attempt_count=1`) != 1 {
				t.Fatal("unknown Child effect retried")
			}
		})
	}
}

func TestChildMigrationPreservesExistingRootWithoutGrantingDelegation(t *testing.T) {
	s, r, a, p := taskFixture(t)
	p.respond = childPolicy
	ctx := context.Background()
	receipt := mustSubmit(t, a, "one", "legacy work")
	main, err := r.Claim(ctx, receipt.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err = executeTaskRun(t, a, r, ctx, main); err != nil {
		t.Fatal(err)
	}
	var rootID, beforeRun, beforeCap string
	if err = s.db.QueryRow(`SELECT r.run_id,r.checkpoint_json,c.tools_json FROM run r JOIN capability_snapshot c ON c.snapshot_id=r.capability_snapshot_id WHERE r.worker_position='root'`).Scan(&rootID, &beforeRun, &beforeCap); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file::memory:?_txlock=immediate&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(ctx, db, migrations[:12]); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`ATTACH DATABASE ? AS prior`, s.path); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"auth_connection", "model_profile", "model_snapshot", "agent_config", "capability_snapshot", "task"} {
		if table == "model_profile" {
			if _, err = db.Exec(`INSERT INTO model_profile SELECT profile_id,name,provider_kind,endpoint,wire_api,model_id,auth_connection_id,reasoning_effort,context_window,max_output_tokens,generation_defaults_json,compatibility_json,capabilities_json,status,verified_at,created_at,updated_at,1 FROM prior.model_profile`); err != nil {
				t.Fatal(err)
			}
			for {
				changed, e := db.Exec(`UPDATE model_profile SET version=version+1 WHERE version<(SELECT p.version FROM prior.model_profile p WHERE p.profile_id=model_profile.profile_id)`)
				if e != nil {
					t.Fatal(e)
				}
				n, _ := changed.RowsAffected()
				if n == 0 {
					break
				}
			}
			continue
		}
		if _, err = db.Exec(`INSERT INTO ` + table + ` SELECT * FROM prior.` + table); err != nil {
			t.Fatal(table, err)
		}
	}
	if _, err = db.Exec(`INSERT INTO run SELECT * FROM prior.run WHERE worker_position='root'`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO run_claim SELECT * FROM prior.run_claim WHERE run_id=?`, rootID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO model_generation(generation_id,run_id,step_index,request_json,request_hash,owner_token,status,created_at) VALUES('old-exchange',?,0,'{}','hash','owner','pending',100)`, rootID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO tool_invocation(invocation_id,run_id,provider_exchange_id,provider_call_id,tool_name,arguments_json,request_hash,replay_policy,idempotency_key,status,delivery_seq,created_at,updated_at,version) VALUES('old-invocation',?,'old-exchange','old-call','builtin__text__echo','{}','hash','safe','key','queued',0,100,100,1)`, rootID); err != nil {
		t.Fatal(err)
	}
	if err = migrate(ctx, db, migrations); err != nil {
		t.Fatal(err)
	}
	var afterRun, afterCap string
	var count, descendants, spawn int
	if err = db.QueryRow(`SELECT r.checkpoint_json,c.tools_json,c.can_spawn_agents,t.tool_calls,t.descendants FROM run r JOIN capability_snapshot c ON c.snapshot_id=r.capability_snapshot_id JOIN run_tree t ON t.root_run_id=r.run_id WHERE r.run_id=?`, rootID).Scan(&afterRun, &afterCap, &spawn, &count, &descendants); err != nil {
		t.Fatal(err)
	}
	if beforeRun != afterRun || beforeCap != afterCap || spawn != 0 || count != 1 || descendants != 0 {
		t.Fatal("upgrade changed frozen Root or lost call count", spawn, count, descendants)
	}
	if err = migrate(ctx, db, migrations); err != nil {
		t.Fatal("repeated upgrade", err)
	}
}

func TestChildStorageRejectsAncestryAndCapabilityExpansion(t *testing.T) {
	s, r, a, p := childFixture(t)
	c, run, gen, calls := childAdmission(t, s, r, a, p, `{"task":"child"}`)
	ctx := context.Background()
	if _, err := r.ToolStore(c).Accept(ctx, gen.Request.GenerationID, calls, run.Tools); err != nil {
		t.Fatal(err)
	}
	var child, taskID, parent, root, capID, modelID, mainID, mainModel string
	var depth int
	if err := s.db.QueryRow(`SELECT r.run_id,r.task_id,r.parent_run_id,r.root_run_id,r.capability_snapshot_id,r.model_snapshot_id,r.depth FROM run r WHERE r.worker_position='child'`).Scan(&child, &taskID, &parent, &root, &capID, &modelID, &depth); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT run_id,model_snapshot_id FROM run WHERE kind='main'`).Scan(&mainID, &mainModel); err != nil {
		t.Fatal(err)
	}
	expanded := newID()
	if _, err := s.db.Exec(`INSERT INTO capability_snapshot SELECT ?,agent_config_version,role,'changed',skill_set_hash,data_scope,can_spawn_agents,json_insert(tools_json,'$[#]',json('{"name":"forbidden-memory-write","permission":"memory_write"}')),created_at,format_version FROM capability_snapshot WHERE snapshot_id=?`, expanded, capID); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, task, parent, root, cap, model string
		depth                                int
	}{
		{"Main parent", taskID, mainID, root, capID, modelID, depth},
		{"different Task", "forged-task", parent, root, capID, modelID, depth},
		{"different Root", taskID, parent, mainID, capID, modelID, depth},
		{"wrong depth", taskID, parent, root, capID, modelID, depth + 1},
		{"new model snapshot", taskID, parent, root, capID, mainModel, depth},
		{"expanded capability", taskID, parent, root, expanded, modelID, depth},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.db.Exec(`INSERT INTO run(run_id,agent_id,conversation_id,task_id,parent_run_id,root_run_id,kind,worker_position,status,task_epoch,depth,capability_snapshot_id,model_snapshot_id,checkpoint_json,history_boundary,created_at,updated_at,version) SELECT ?,agent_id,conversation_id,?,?,?,kind,worker_position,'queued',task_epoch,?,?,?,checkpoint_json,history_boundary,created_at,updated_at,1 FROM run WHERE run_id=?`, newID(), tc.task, tc.parent, tc.root, tc.depth, tc.cap, tc.model, child)
			if err == nil {
				t.Fatal("accepted invalid Child inheritance")
			}
		})
	}
	if scalar(t, s, `SELECT count(*) FROM run WHERE worker_position='child'`) != 1 {
		t.Fatal("invalid Child persisted")
	}
}

func TestChildDoesNotReceiveAppendedTaskInstructions(t *testing.T) {
	s, r, a, p := childFixture(t)
	c, run, gen, calls := childAdmission(t, s, r, a, p, `{"task":"child"}`)
	ctx := context.Background()
	if _, err := r.ToolStore(c).Accept(ctx, gen.Request.GenerationID, calls, run.Tools); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO task_instruction(instruction_id,task_id,task_epoch,instruction,artifact_refs,created_at) SELECT 'appended',task_id,task_epoch,'private appended Task instruction','[]',100 FROM task`); err != nil {
		t.Fatal(err)
	}
	var child string
	if err := s.db.QueryRow(`SELECT child_run_id FROM agent_invocation`).Scan(&child); err != nil {
		t.Fatal(err)
	}
	cc, err := r.Claim(ctx, child)
	if err != nil {
		t.Fatal(err)
	}
	inputs, err := r.Steering(ctx, cc, 0)
	if err != nil || len(inputs) != 0 {
		t.Fatal("Child inherited mutable Task steering", inputs, err)
	}
	cr, err := r.Load(ctx, cc)
	if err != nil {
		t.Fatal(err)
	}
	cg, err := r.Generation(ctx, cc, cr)
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(cg.Request.Messages)
	if strings.Contains(string(wire), "private appended") {
		t.Fatal("Child model received Task mutation")
	}
}

func TestChildOutOfOrderCompletionBeforeParentWaitKeepsCallOrder(t *testing.T) {
	s, r, a, p := childFixture(t)
	c, run, gen, calls := childAdmission(t, s, r, a, p, `{"task":"first child result"}`, `{"task":"second child result"}`)
	ctx := context.Background()
	ts := r.ToolStore(c)
	invs, err := ts.Accept(ctx, gen.Request.GenerationID, calls, run.Tools)
	if err != nil {
		t.Fatal(err)
	}
	p.respond = func(req llm.GenerationRequest) (llm.GenerationResult, error) {
		return llm.GenerationResult{Text: req.Messages[len(req.Messages)-1].Text}, nil
	}
	for _, index := range []int{1, 0} {
		var id string
		if err = s.db.QueryRow(`SELECT child_run_id FROM agent_invocation WHERE invocation_id=?`, invs[index].ID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		cc, e := r.Claim(ctx, id)
		if e != nil {
			t.Fatal(e)
		}
		if e = executeTaskRun(t, a, r, ctx, cc); e != nil {
			t.Fatal(e)
		}
	}
	selected, err := a.Registry.Select(run.Tools)
	if err != nil {
		t.Fatal(err)
	}
	e := tool.Executor{Registry: selected, Store: ts, Concurrency: 4}
	results, err := e.Execute(ctx, gen.Request.GenerationID, calls)
	if err != nil || len(results) != 2 {
		t.Fatal(results, err)
	}
	for i, result := range results {
		var value agent.AgentResult
		if json.Unmarshal([]byte(result.ModelText()), &value) != nil || value.Text != []string{"first child result", "second child result"}[i] {
			t.Fatal("completion order changed call association", results)
		}
	}
	again, err := e.Execute(ctx, gen.Request.GenerationID, calls)
	if err != nil || !reflect.DeepEqual(results, again) {
		t.Fatal("completed result was not reused", again, err)
	}
	if err = executeTaskRun(t, a, r, ctx, c); err != nil {
		t.Fatal(err)
	}
	if scalar(t, s, `SELECT descendants FROM run_tree`) != 2 || scalar(t, s, `SELECT count(*) FROM task_update`) != 1 {
		t.Fatal("duplicate wake or lost completion")
	}
}

func TestChildLargeResultUsesSharedArtifactNormalizationAndKeepsEnvelope(t *testing.T) {
	s, _, a, p := childFixture(t)
	input := modelInput()
	input.ModelID = "large-output"
	input.MaxOutputTokens = 8192
	activateLoop(t, s, input)
	large := strings.Repeat("bounded detail ", 1800)
	p.respond = func(req llm.GenerationRequest) (llm.GenerationResult, error) {
		last := req.Messages[len(req.Messages)-1]
		if !isRoot(req) && !isFollowup(req) && last.Role != "tool" {
			return calls(task.ManageName, `{"operation":"create","task":"large result root"}`), nil
		}
		if last.Role == "tool" || isFollowup(req) {
			return llm.GenerationResult{Text: "Result saved as an Artifact"}, nil
		}
		if strings.Contains(last.Text, "Task anchor:") {
			return calls(agent.RunToolName, `{"task":"large child"}`), nil
		}
		return llm.GenerationResult{Text: large}, nil
	}
	mustSubmit(t, a, "one", "delegate")
	mustDrain(t, a)
	var raw, parent, taskID string
	if err := s.db.QueryRow(`SELECT i.result_json,i.run_id,r.task_id FROM tool_invocation i JOIN run r USING(run_id) WHERE i.tool_name=?`, agent.RunToolName).Scan(&raw, &parent, &taskID); err != nil {
		t.Fatal(err)
	}
	var result tool.Result
	var value agent.AgentResult
	if json.Unmarshal([]byte(raw), &result) != nil || json.Unmarshal([]byte(result.ModelText()), &value) != nil || len(value.ArtifactRefs) != 1 || len(value.Text) > 16384 || len(result.ArtifactRefs) != 0 {
		t.Fatal("large Agent result changed API shape", raw)
	}
	scope := artifact.Scope{UserID: artifact.LocalUser, AgentID: idAgent(t, s), TaskID: taskID, RunID: parent}
	if readArtifact(t, s, scope, value.ArtifactRefs[0]) != large {
		t.Fatal("Child full result lost")
	}
	if scalar(t, s, `SELECT count(*) FROM artifact`) != 1 || scalar(t, s, `SELECT count(*) FROM task WHERE status='completed'`) != 1 || scalar(t, s, `SELECT count(*) FROM artifact_publication`) != 0 {
		t.Fatal("large output duplicated, overflowed Parent context or auto-published")
	}
	for _, req := range p.requests {
		if req.RunID == parent {
			for _, m := range req.Messages {
				if strings.Contains(m.Text, large) {
					t.Fatal("large Child result entered Parent model context")
				}
			}
		}
	}
}
