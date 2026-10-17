package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/z-chenhao/erii/internal/agent"
	"github.com/z-chenhao/erii/internal/llm"
	"github.com/z-chenhao/erii/internal/tool"
	"github.com/z-chenhao/erii/internal/tool/codemode"
	toolfake "github.com/z-chenhao/erii/internal/tool/fake"
)

func codeClaimFixture(t *testing.T) (*Store, *Runs, *codeBridge, tool.Invocation) {
	t.Helper()
	echo, _ := tool.EchoRegistry().Resolve("builtin__text__echo")
	calls := []llm.ToolCall{{ID: "code", Name: codemode.ToolName, Arguments: `{"source":"return 1;"}`}}
	s, r, _, e, exchange := admitPolicy(t, []tool.Definition{codemode.Definition(), echo}, calls)
	invs, err := e.Store.Accept(context.Background(), exchange, calls, e.Registry.Describe())
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Store.Start(context.Background(), invs[0].ID); err != nil {
		t.Fatal(err)
	}
	b := &codeBridge{t: e.Store.(*tools), registry: e.Registry, attempt: newID(), req: codemode.ExecutionRequest{ProtocolVersion: codemode.ProtocolVersion, RuntimeVersion: codemode.RuntimeVersion, PreludeVersion: codemode.PreludeVersion, ExecutionID: invs[0].ID, ParentInvocationID: invs[0].ID, RunID: e.Store.(*tools).claim.RunID(), Source: "return 1;", SourceHash: digest("return 1;"), Limits: codemode.DefaultLimits()}}
	if err = b.claim(context.Background(), invs[0]); err != nil {
		t.Fatal(err)
	}
	return s, r, b, invs[0]
}
func codeMessage(b *codeBridge) codemode.Message {
	return codemode.Message{ExecutionID: b.req.ExecutionID, ParentInvocationID: b.req.ParentInvocationID, RunID: b.req.RunID, ClaimToken: b.req.ClaimToken, ExecutionEpoch: b.req.ExecutionEpoch, Sequence: 1, CallID: "1", CallKey: strings.Repeat("a", 64), ToolName: "builtin__text__echo", Arguments: json.RawMessage(`{"text":"first"}`)}
}
func TestCodeReplayFrozenRequestAndAtomicAdmission(t *testing.T) {
	s, _, b, _ := codeClaimFixture(t)
	ctx := context.Background()
	m := codeMessage(b)
	for i := 0; i < 2; i++ {
		reply, err := b.Handle(ctx, "host.invoke", m)
		if err != nil || !reply.Suspended {
			t.Fatalf("admission %+v %v", reply, err)
		}
	}
	if scalar(t, s, `SELECT count(*) FROM code_call`) != 1 || scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE parent_invocation_id IS NOT NULL`) != 1 {
		t.Fatal("duplicate admission")
	}
	m.Sequence = 2
	m.Arguments = json.RawMessage(`{"text":"changed"}`)
	reply, err := b.Handle(ctx, "host.invoke", m)
	if err != nil || reply.Error == nil || reply.Error.Code != "NON_DETERMINISTIC_REPLAY" {
		t.Fatalf("drift %+v %v", reply, err)
	}
	m.Sequence = 3
	m.Arguments = json.RawMessage(`{"text":"first"}`)
	if reply, err = b.Handle(ctx, "host.invoke", m); err != nil || !reply.Suspended {
		t.Fatalf("original parameters did not reattach: %+v %v", reply, err)
	}
	specs := b.registry.Describe()
	specs[0].InputSchema = `{"type":"object"}`
	_, err = (nestedTools{b.t.forCode(b), b.req.ExecutionID}).Accept(ctx, b.exchange, []llm.ToolCall{{ID: b.req.ExecutionID + "/" + m.CallKey, Name: m.ToolName, Arguments: `{"text":"first"}`}}, specs)
	if !errors.Is(err, tool.ErrReplay) {
		t.Fatalf("schema drift %v", err)
	}
	if scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE parent_invocation_id IS NOT NULL AND attempt_count>0`) != 0 {
		t.Fatal("admission dispatched effect")
	}
}
func TestCodeClaimWakeAndOldCallbackFencing(t *testing.T) {
	s, r, b, inv := codeClaimFixture(t)
	ctx := context.Background()
	observed, err := b.wake(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`UPDATE code_execution_state SET wake_seq=wake_seq+1,version=version+1`); err != nil {
		t.Fatal(err)
	}
	idle, err := b.release(ctx, observed)
	if err != nil || idle {
		t.Fatalf("lost wake %v %v", idle, err)
	}
	observed, _ = b.wake(ctx)
	idle, err = b.release(ctx, observed)
	if err != nil || !idle {
		t.Fatalf("release %v %v", idle, err)
	}
	if err = b.t.Start(ctx, inv.ID); err != nil {
		t.Fatal(err)
	}
	next := &codeBridge{t: b.t, req: b.req, attempt: newID(), registry: b.registry}
	next.req.ClaimToken = ""
	next.req.ExecutionEpoch = 0
	if err = next.claim(ctx, inv); err != nil {
		t.Fatal(err)
	}
	if next.req.ExecutionEpoch != b.req.ExecutionEpoch+1 {
		t.Fatal("epoch did not increase")
	}
	// A late completion from an older VM must also fail inside the ordinary
	// nested Tool pipeline, not just at the RPC/event admission boundary.
	staleStore := b.t.forCode(b)
	if err = staleStore.Finish(ctx, inv.ID, tool.Result{Text: "late"}); !errors.Is(err, agent.ErrClaimLost) {
		t.Fatalf("old terminal callback %v", err)
	}
	staleResult := tool.Result{Text: "late wrapper", HostFence: codeClaimFence{b.req.ClaimToken, b.req.ExecutionEpoch}}
	if err = b.t.Finish(ctx, inv.ID, staleResult); !errors.Is(err, agent.ErrClaimLost) {
		t.Fatalf("old wrapper completion %v", err)
	}
	old := codeMessage(b)
	if _, err = b.Handle(ctx, "host.invoke", old); !errors.Is(err, agent.ErrClaimLost) {
		t.Fatalf("old callback %v", err)
	}
	if scalar(t, s, `SELECT count(*) FROM code_call`) != 0 {
		t.Fatal("stale owner wrote ledger")
	}
	// An expired execution lease is scanned even if no queue event survives.
	if _, err = s.db.Exec(`UPDATE code_execution_state SET claim_expires_at=0,version=version+1`); err != nil {
		t.Fatal(err)
	}
	if err = r.recoverCode(ctx); err != nil {
		t.Fatal(err)
	}
	if scalar(t, s, `SELECT count(*) FROM code_execution_state WHERE claim_token IS NULL AND state='suspended'`) != 1 {
		t.Fatal("expired claim not recovered")
	}
	if _, err = next.Handle(ctx, "host.invoke", codeMessage(next)); !errors.Is(err, agent.ErrClaimLost) {
		t.Fatalf("expired callback %v", err)
	}
}
func TestCodeDeliveryOrderIsDurable(t *testing.T) {
	s, _, b, _ := codeClaimFixture(t)
	ctx := context.Background()
	second, err := b.deliver(ctx, "second", codemode.Reply{Value: "second finished first"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := b.deliver(ctx, "first", codemode.Reply{Value: "first finished second"})
	if err != nil {
		t.Fatal(err)
	}
	if second.DeliverySeq != 1 || first.DeliverySeq != 2 {
		t.Fatalf("%+v %+v", first, second)
	}
	for _, key := range []string{"first", "second"} {
		replayed, err := b.deliver(ctx, key, codemode.Reply{Value: "not a replacement"})
		if err != nil {
			t.Fatal(err)
		}
		if replayed.Value == "not a replacement" {
			t.Fatal("replaced frozen result")
		}
	}
	if scalar(t, s, `SELECT count(*) FROM code_delivery`) != 2 {
		t.Fatal("duplicate delivery")
	}
}
func TestCodeManifestCannotChangeDuringRecovery(t *testing.T) {
	s, _, b, inv := codeClaimFixture(t)
	ctx := context.Background()
	if _, err := s.db.Exec(`UPDATE code_manifest SET source='changed'`); err == nil {
		t.Fatal("mutable source")
	}
	if _, err := s.db.Exec(`UPDATE code_execution SET runtime_version='changed'`); err == nil {
		t.Fatal("mutable runtime")
	}
	if err := b.abandon(); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"source", "runtime", "prelude", "runtime-bytes", "prelude-bytes", "input-limits"} {
		next := &codeBridge{t: b.t, req: b.req, registry: b.registry}
		next.req.ClaimToken = ""
		next.req.ExecutionEpoch = 0
		switch field {
		case "source":
			next.req.Source = "return 2;"
			next.req.SourceHash = digest(next.req.Source)
		case "runtime":
			next.req.RuntimeVersion = "changed"
		case "prelude":
			next.req.PreludeVersion = "changed"
		case "runtime-bytes":
			next.req.RuntimeHash = "changed"
		case "prelude-bytes":
			next.req.PreludeHash = "changed"
		case "input-limits":
			next.req.Limits.TimeoutMS++
		}
		if err := next.claim(ctx, inv); !errors.Is(err, tool.ErrReplay) {
			t.Fatalf("%s drift: %v", field, err)
		}
		if err := next.abandon(); err != nil {
			t.Fatal(err)
		}
	}
}
func TestCodeRecoveryMigrationV16RollbackAndPreservation(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(ctx, db, migrations[:16]); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO agent_config VALUES(1,'prior',NULL,'setup_required',7,1,1)`); err != nil {
		t.Fatal(err)
	}
	if err = migrate(ctx, db, append(append([]string{}, migrations[:16]...), codeRecoverySQL+`; INVALID SQL;`)); err == nil {
		t.Fatal("accepted broken migration")
	}
	var version int
	db.QueryRow(`PRAGMA user_version`).Scan(&version)
	if version != 16 {
		t.Fatal(version)
	}
	if err = migrate(ctx, db, migrations); err != nil {
		t.Fatal(err)
	}
	var identity string
	if err = db.QueryRow(`SELECT agent_id FROM agent_config`).Scan(&identity); err != nil || identity != "prior" {
		t.Fatalf("preservation %s %v", identity, err)
	}
}
func TestHoursLongApprovalHoldsNoJavaScriptRuntime(t *testing.T) {
	effect := toolfake.Definition(t.TempDir(), "query_before_retry", nil)
	s, r, a, p := childFixture(t, codemode.Definition(), effect)
	r.ConfigureJavaScript(realCodeRuntime(t))
	r.ConfigureApprovalTTL(12 * time.Hour)
	clock := time.Now()
	r.approvalClock = func() time.Time { return clock }
	p.respond = func(req llm.GenerationRequest) (llm.GenerationResult, error) {
		if req.Messages[len(req.Messages)-1].Role == "tool" {
			return llm.GenerationResult{Text: "observed"}, nil
		}
		raw, _ := json.Marshal(map[string]string{"source": `return await tools.mcp__fixture__send({target:"clock",text:"six hours"});`})
		return calls(codemode.ToolName, string(raw)), nil
	}
	mustSubmit(t, a, "long-approval", "execute")
	mustDrain(t, a)
	clock = clock.Add(6 * time.Hour)
	approvals, err := r.Approvals(context.Background())
	if err != nil || len(approvals) != 1 || approvals[0].Status != "pending" {
		t.Fatalf("%+v %v", approvals, err)
	}
	if scalar(t, s, `SELECT count(*) FROM code_execution_state WHERE state='suspended' AND claim_token IS NULL`) != 1 {
		t.Fatal("waiting execution retained claim")
	}
	if scalar(t, s, `SELECT count(*) FROM code_event WHERE json_extract(message_json,'$.kind')='execution_suspended' AND json_extract(message_json,'$.metrics.activeVMs')=0`) == 0 {
		t.Fatal("VM was not disposed")
	}
	if _, err = r.Next(context.Background()); !errors.Is(err, agent.ErrNotFound) {
		t.Fatalf("approval retained runnable work: %v", err)
	}
	if err = r.Decide(context.Background(), idAgent(t, s), approvals[0].ID, approvals[0].Hash, "approved"); err != nil {
		t.Fatal(err)
	}
	mustDrain(t, a)
	if scalar(t, s, `SELECT count(*) FROM code_execution_state WHERE state='completed' AND claim_token IS NULL`) != 1 {
		t.Fatal("approval could not resume")
	}
}

func TestCodeReplayStoreObservationsAndWritesAreNotRepeated(t *testing.T) {
	s, r, a, p := childFixture(t, codemode.Definition())
	r.ConfigureJavaScript(realCodeRuntime(t))
	p.respond = func(req llm.GenerationRequest) (llm.GenerationResult, error) {
		if req.Messages[len(req.Messages)-1].Role == "tool" {
			return llm.GenerationResult{Text: "observed"}, nil
		}
		raw, _ := json.Marshal(map[string]string{"source": `store("count",(load("count")||0)+1); await tools.builtin__text__echo({text:String(load("count"))}); return load("count");`})
		return calls(codemode.ToolName, string(raw)), nil
	}
	mustSubmit(t, a, "store-replay", "execute")
	mustDrain(t, a)
	if scalar(t, s, `SELECT count(*) FROM code_store WHERE value_json='1' AND version=1`) != 1 {
		t.Fatal("replay repeated a Store write")
	}
	var raw string
	if err := s.db.QueryRow(`SELECT result_json FROM tool_invocation WHERE tool_name=?`, codemode.ToolName).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var result tool.Result
	if json.Unmarshal([]byte(raw), &result) != nil || result.Text != `[{"type":"return","value":1}]` {
		t.Fatalf("Store changed replay: %s", raw)
	}
}
