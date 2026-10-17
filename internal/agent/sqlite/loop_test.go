package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/z-chenhao/erii/internal/agent"
	"github.com/z-chenhao/erii/internal/conversation"
	conversationsqlite "github.com/z-chenhao/erii/internal/conversation/sqlite"
	"github.com/z-chenhao/erii/internal/llm"
	"github.com/z-chenhao/erii/internal/tool"
)

type loopProvider struct {
	store    *Store
	mu       sync.Mutex
	requests []llm.GenerationRequest
	respond  func(llm.GenerationRequest) (llm.GenerationResult, error)
}

func (p *loopProvider) Generate(ctx context.Context, r llm.GenerationRequest, sink func(llm.GenerationEvent)) (llm.GenerationResult, error) {
	p.mu.Lock()
	p.requests = append(p.requests, r)
	p.mu.Unlock()
	result, err := p.respond(r)
	if accepted := p.store.Models().RecordUsageAnd(ctx, r, result, time.Millisecond, err, p.store.AcceptGeneration); accepted != nil {
		return llm.GenerationResult{}, accepted
	}
	if err == nil && sink != nil {
		sink(llm.GenerationEvent{Kind: "completed", GenerationID: r.GenerationID})
	}
	return result, err
}
func (p *loopProvider) Continue(ctx context.Context, r llm.GenerationRequest, sink func(llm.GenerationEvent)) (llm.GenerationResult, error) {
	return p.Generate(ctx, r, sink)
}
func (p *loopProvider) Cancel(context.Context, string) error  { return nil }
func (p *loopProvider) Dispose(context.Context, string) error { return nil }

func activateLoop(t *testing.T, s *Store, input llm.ProfileInput) llm.Profile {
	t.Helper()
	ctx := context.Background()
	p, err := s.Models().AddProfile(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	c := control(s)
	if _, err = c.Test(ctx, p.ProfileID); err != nil {
		t.Fatal(err)
	}
	d, err := s.Diagnostics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Select(ctx, p.ProfileID, p.Version, d.ConfigVersion); err != nil {
		t.Fatal(err)
	}
	p, err = s.Models().Profile(ctx, p.ProfileID)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func loopFixture(t *testing.T) (*Store, *Runs, *conversationsqlite.Store, agent.Actor, *loopProvider) {
	t.Helper()
	s := openModels(t)
	activateLoop(t, s, modelInput())
	h, err := conversationsqlite.Open(context.Background(), filepath.Dir(s.path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	registry := tool.EchoRegistry()
	r := s.Runs(h, registry)
	p := &loopProvider{store: s, respond: func(request llm.GenerationRequest) (llm.GenerationResult, error) {
		last := request.Messages[len(request.Messages)-1]
		if last.Role == "tool" {
			return llm.GenerationResult{Text: "result: " + last.Text}, nil
		}
		if last.Role != "user" {
			return llm.GenerationResult{}, llm.InvalidProviderResponse
		}
		args, _ := json.Marshal(struct {
			Text string `json:"text"`
		}{last.Text})
		return llm.GenerationResult{ToolCalls: []llm.ToolCall{{ID: "same-id", Name: "builtin__text__echo", Arguments: string(args)}}}, nil
	}}
	a := agent.Actor{Store: r, Provider: p, Registry: registry, Ready: func(context.Context) bool { return true }}
	return s, r, h, a, p
}
func mustSubmit(t *testing.T, a agent.Actor, key, text string) agent.Receipt {
	t.Helper()
	receipt, err := a.Submit(context.Background(), key, text)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}
func mustDrain(t *testing.T, a agent.Actor) {
	t.Helper()
	if err := a.Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func scalar(t *testing.T, s *Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLoopConcurrentIngressOneMainAndIdempotentConversation(t *testing.T) {
	s, r, h, a, p := loopFixture(t)
	ctx := context.Background()
	other, err := Open(ctx, filepath.Dir(s.path))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	a2 := a
	a2.Store = other.Runs(h, a.Registry)
	var wg sync.WaitGroup
	receipts := make(chan agent.Receipt, 32)
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			actor := a
			if i%2 == 1 {
				actor = a2
			}
			receipt, err := actor.Submit(ctx, fmt.Sprint(i), fmt.Sprintf("message %d", i))
			receipts <- receipt
			errs <- err
		}(i)
	}
	wg.Wait()
	close(receipts)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	id := ""
	for receipt := range receipts {
		if id == "" {
			id = receipt.RunID
		}
		if receipt.RunID != id {
			t.Fatal("multiple Main Runs")
		}
	}
	if scalar(t, s, `SELECT count(*) FROM run`) != 1 {
		t.Fatal("multiple persisted runs")
	}
	replayed := mustSubmit(t, a, "0", "message 0")
	if replayed.RunID != id {
		t.Fatal("retry created a new run")
	}
	if _, err := a.Submit(ctx, "0", "changed"); !errors.Is(err, conversation.ErrConflict) {
		t.Fatal("accepted parameter drift", err)
	}
	mustDrain(t, a)
	if err = r.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	messages, err := h.List(ctx, idAgent(t, s))
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 33 {
		t.Fatal("semantic message count", len(messages))
	}
	for i, m := range messages {
		if m.Sequence != int64(i+1) {
			t.Fatal("sequence gap")
		}
		if i < 32 && m.Role != "user" {
			t.Fatal("trace leaked into conversation")
		}
	}
	if len(p.requests) != 2 || scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE status='succeeded'`) != 1 || scalar(t, s, `SELECT count(*) FROM delivery_request`) != 1 {
		t.Fatal("wrong provider/tool/delivery counts")
	}
	if scalar(t, s, `SELECT count(*) FROM run WHERE status='completed'`) != 1 {
		t.Fatal("run not complete")
	}
	before := len(p.requests)
	mustDrain(t, a)
	if len(p.requests) != before {
		t.Fatal("terminal run replayed")
	}
	for _, query := range []string{`UPDATE run SET status='running',ended_at=NULL,version=version+1`, `DELETE FROM run`, `UPDATE capability_snapshot SET can_spawn_agents=1`, `DELETE FROM capability_snapshot`, `UPDATE model_generation SET status='pending'`, `UPDATE tool_invocation SET status='running',version=version+1`} {
		if _, err = s.db.Exec(query); err == nil {
			t.Fatal("mutated frozen or terminal record", query)
		}
	}
}
func idAgent(t *testing.T, s *Store) string {
	t.Helper()
	d, err := s.Diagnostics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return d.AgentID
}

func TestLoopSteeringAtBoundaryAndProfileSwitchOnlyNewRuns(t *testing.T) {
	s, r, h, a, p := loopFixture(t)
	first := mustSubmit(t, a, "first", "original")
	entered, release := make(chan struct{}), make(chan struct{})
	original := p.respond
	once := sync.Once{}
	p.respond = func(request llm.GenerationRequest) (llm.GenerationResult, error) {
		once.Do(func() { close(entered); <-release })
		return original(request)
	}
	done := make(chan error, 1)
	go func() { done <- a.Drain(context.Background()) }()
	<-entered
	second := mustSubmit(t, a, "second", "steer one")
	third := mustSubmit(t, a, "third", "steer two")
	if second.RunID != first.RunID || third.RunID != first.RunID {
		t.Fatal("busy Main spawned another run")
	}
	input := modelInput()
	input.ModelID = "new-model"
	input.Endpoint = "http://127.0.0.1:9876/v1"
	newProfile := activateLoop(t, s, input)
	if _, err := r.Claim(context.Background(), first.RunID); !errors.Is(err, agent.ErrBusy) {
		t.Fatal("stole live claim", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(p.requests) != 3 {
		t.Fatal("steering was not batched at tool boundary", len(p.requests))
	}
	for _, request := range p.requests {
		if request.Snapshot.ProfileID == newProfile.ProfileID {
			t.Fatal("active Run switched model")
		}
	}
	last := p.requests[1].Messages
	if last[len(last)-2].Text != "steer one" || last[len(last)-1].Text != "steer two" {
		t.Fatal("steering joined, reordered or lost")
	}
	if scalar(t, s, `SELECT count(*) FROM delivery_request`) != 1 {
		t.Fatal("stale generation delivered")
	}
	if scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE provider_call_id='same-id' AND status='succeeded'`) != 2 {
		t.Fatal("exchange-local call IDs collided")
	}
	next := mustSubmit(t, a, "fourth", "new run")
	if next.RunID == first.RunID {
		t.Fatal("revived completed run")
	}
	mustDrain(t, a)
	if p.requests[len(p.requests)-1].Snapshot.ProfileID != newProfile.ProfileID {
		t.Fatal("new run did not use new model")
	}
	if err := r.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	messages, err := h.List(context.Background(), idAgent(t, s))
	if err != nil || len(messages) != 6 {
		t.Fatal("history", len(messages), err)
	}
}

func TestLoopOldClaimsFenceEveryMutationAndModelAcceptance(t *testing.T) {
	s, r, _, a, _ := loopFixture(t)
	ctx := context.Background()
	receipt := mustSubmit(t, a, "first", "input")
	old, err := r.Claim(ctx, receipt.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Initialize(ctx, old); err != nil {
		t.Fatal(err)
	}
	run, err := r.Load(ctx, old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Complete(ctx, old, run.Version); !errors.Is(err, agent.ErrConflict) {
		t.Fatal("completed before semantic delivery", err)
	}
	inputs, err := r.Steering(ctx, old, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range inputs {
		run.Checkpoint.Messages = append(run.Checkpoint.Messages, agent.AgentMessage{Type: "user", Content: i.Message.Content})
		run.Checkpoint.InputSequence = i.Sequence
	}
	if err = r.Checkpoint(ctx, old, run.Version, run.Checkpoint); err != nil {
		t.Fatal(err)
	}
	run, _ = r.Load(ctx, old)
	generation, err := r.Generation(ctx, old, run)
	if err != nil {
		t.Fatal(err)
	}
	// A fixture clock advance affects only the exact test Run's lease.
	if _, err = s.db.Exec(`UPDATE run_claim SET expires_at=0 WHERE run_id=?`, receipt.RunID); err != nil {
		t.Fatal(err)
	}
	current, err := r.Claim(ctx, receipt.RunID)
	if err != nil {
		t.Fatal(err)
	}
	checks := []func() error{
		func() error { _, err := r.StopRequested(ctx, old); return err },
		func() error { _, err := r.SettleStop(ctx, old); return err },
		func() error { return r.Renew(ctx, old) }, func() error { return r.Release(ctx, old) }, func() error { return r.Checkpoint(ctx, old, run.Version, run.Checkpoint) }, func() error { return r.Wait(ctx, old, run.Version, "resource") }, func() error { return r.Fail(ctx, old, run.Version, "PROVIDER_UNAVAILABLE") }, func() error { _, err := r.Complete(ctx, old, run.Version); return err }, func() error {
			_, err := r.ToolStore(old).Accept(ctx, generation.Request.GenerationID, nil, a.Registry.Describe())
			return err
		},
		func() error {
			return s.Models().RecordUsageAnd(ctx, generation.Request, llm.GenerationResult{Text: "late"}, 0, nil, s.AcceptGeneration)
		},
		func() error { return r.ToolStore(old).Start(ctx, "not-found") },
		func() error { return r.ToolStore(old).Finish(ctx, "not-found", tool.Result{Text: "late"}) },
		func() error { _, err := r.PrepareReply(ctx, old, run); return err },
		func() error { return r.DeliverReply(ctx, old, run) },
	}
	for i, check := range checks {
		if err := check(); !errors.Is(err, agent.ErrClaimLost) {
			t.Fatalf("old claim write %d: %v", i, err)
		}
	}
	if scalar(t, s, `SELECT count(*) FROM provider_usage`) != 0 {
		t.Fatal("late completion committed usage")
	}
	currentRun, err := r.Load(ctx, current)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Checkpoint(ctx, current, currentRun.Version-1, currentRun.Checkpoint); !errors.Is(err, agent.ErrConflict) {
		t.Fatal("CAS accepted stale version", err)
	}
	if err = r.Wait(ctx, current, currentRun.Version, "invalid"); !errors.Is(err, agent.ErrConflict) {
		t.Fatal("illegal wait reason")
	}
	if err = r.Wait(ctx, current, currentRun.Version, "resource"); err != nil {
		t.Fatal(err)
	}
	reclaimed, err := r.Claim(ctx, receipt.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Release(ctx, reclaimed); err != nil {
		t.Fatal(err)
	}
}

func TestLoopToolRejectionPersistsAndDoesNotExposeHostData(t *testing.T) {
	s, _, _, a, p := loopFixture(t)
	p.respond = func(request llm.GenerationRequest) (llm.GenerationResult, error) {
		last := request.Messages[len(request.Messages)-1]
		if last.Role == "tool" {
			return llm.GenerationResult{Text: last.Text}, nil
		}
		return llm.GenerationResult{ToolCalls: []llm.ToolCall{{ID: "bad", Name: "builtin__text__echo", Arguments: `{"text":false}`}, {ID: "unknown", Name: "builtin__agent__run", Arguments: `{"task":"forbidden"}`}}}, nil
	}
	mustSubmit(t, a, "one", "try invalid calls")
	mustDrain(t, a)
	if scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE status='failed'`) != 2 {
		t.Fatal("invalid/absent Tool not durably rejected")
	}
	var raw string
	if err := s.db.QueryRow(`SELECT result_json FROM tool_invocation WHERE provider_call_id='unknown'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var result tool.Result
	if json.Unmarshal([]byte(raw), &result) != nil || result.Error == nil || result.Error.Code != "NOT_FOUND" || result.Error.Retryable {
		t.Fatal("unstable Tool error", raw)
	}
	if scalar(t, s, `SELECT count(*) FROM run`) != 1 {
		t.Fatal("Main created Child")
	}
}

func TestLoopModelAndReplyPersistenceFailureRecoverWithoutDuplicate(t *testing.T) {
	for _, boundary := range []string{"model_generation.completed", "run.checkpointed"} {
		t.Run(boundary, func(t *testing.T) {
			s, r, _, a, p := loopFixture(t)
			mustSubmit(t, a, "one", "hello")
			// Outbox failure proves domain/result writes roll back with the event.
			query := fmt.Sprintf(`CREATE TRIGGER fail_main BEFORE INSERT ON event_outbox WHEN NEW.type='ai.erii.%s.v1' BEGIN SELECT RAISE(ABORT,'injected'); END`, boundary)
			if boundary == "run.checkpointed" {
				query = `CREATE TRIGGER fail_main BEFORE INSERT ON event_outbox WHEN NEW.type='ai.erii.run.checkpointed.v1' AND EXISTS(SELECT 1 FROM model_generation WHERE status='completed') BEGIN SELECT RAISE(ABORT,'injected'); END`
			}
			if _, err := s.db.Exec(query); err != nil {
				t.Fatal(err)
			}
			if err := a.Drain(context.Background()); !errors.Is(err, agent.ErrStorage) {
				t.Fatal("storage failure swallowed", err)
			}
			if scalar(t, s, `SELECT count(*) FROM delivery_request`) != 0 {
				t.Fatal("delivered before durable writes")
			}
			if boundary == "model_generation.completed" && scalar(t, s, `SELECT count(*) FROM provider_usage`) != 0 {
				t.Fatal("partial model acceptance")
			}
			if _, err := s.db.Exec(`DROP TRIGGER fail_main`); err != nil {
				t.Fatal(err)
			}
			mustDrain(t, a)
			if boundary == "run.checkpointed" && len(p.requests) != 2 {
				t.Fatal("replayed accepted generation", len(p.requests))
			}
			if err := r.Flush(context.Background()); err != nil {
				t.Fatal(err)
			}
			if scalar(t, s, `SELECT count(*) FROM delivery_request`) != 1 {
				t.Fatal("delivery count")
			}
			before := len(p.requests)
			mustDrain(t, a)
			if len(p.requests) != before {
				t.Fatal("replayed terminal result")
			}
		})
	}
}

func TestLoopShutdownPreservesAcceptedGeneration(t *testing.T) {
	s, r, _, a, p := loopFixture(t)
	receipt := mustSubmit(t, a, "one", "hello")
	ctx, stop := context.WithCancel(context.Background())
	original := p.respond
	p.respond = func(request llm.GenerationRequest) (llm.GenerationResult, error) { stop(); return original(request) }
	if err := a.Drain(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("shutdown", err)
	}
	if scalar(t, s, `SELECT count(*) FROM run WHERE run_id=? AND status='running'`, receipt.RunID) != 1 || scalar(t, s, `SELECT count(*) FROM model_generation WHERE status='completed'`) != 1 {
		t.Fatal("shutdown cancelled business or lost accepted result")
	}
	p.respond = original
	mustDrain(t, a)
	if len(p.requests) != 2 {
		t.Fatal("completed generation repeated", len(p.requests))
	}
	if err := r.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLoopModelFailureIsTerminalObservableAndNotRetried(t *testing.T) {
	s, _, _, a, p := loopFixture(t)
	p.respond = func(llm.GenerationRequest) (llm.GenerationResult, error) {
		return llm.GenerationResult{}, llm.ProviderUnavailable
	}
	mustSubmit(t, a, "one", "hello")
	if err := a.Drain(context.Background()); err != llm.ProviderUnavailable {
		t.Fatal("model failure not observable", err)
	}
	if scalar(t, s, `SELECT count(*) FROM run WHERE status='failed' AND error_code='PROVIDER_UNAVAILABLE'`) != 1 {
		t.Fatal("failure not durable")
	}
	if scalar(t, s, `SELECT count(*) FROM delivery_request`) != 0 {
		t.Fatal("failed model delivered a reply")
	}
	mustDrain(t, a)
	if len(p.requests) != 1 {
		t.Fatal("Host retried a failed model")
	}
}

func TestLoopSteeringOvertakesFinalGenerationBeforeReplyCommit(t *testing.T) {
	s, r, h, a, p := loopFixture(t)
	mustSubmit(t, a, "one", "original")
	entered, release := make(chan struct{}), make(chan struct{})
	once := sync.Once{}
	p.respond = func(request llm.GenerationRequest) (llm.GenerationResult, error) {
		first := false
		once.Do(func() { first = true; close(entered); <-release })
		if first {
			return llm.GenerationResult{Text: "stale final"}, nil
		}
		return llm.GenerationResult{Text: "steered final"}, nil
	}
	done := make(chan error, 1)
	go func() { done <- a.Drain(context.Background()) }()
	<-entered
	mustSubmit(t, a, "two", "new direction")
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := r.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	messages, err := h.List(context.Background(), idAgent(t, s))
	if err != nil || len(messages) != 3 || messages[2].Content[0].Text != "steered final" {
		t.Fatal("stale final escaped steering", messages, err)
	}
	if len(p.requests) != 2 || scalar(t, s, `SELECT count(*) FROM delivery_request`) != 1 {
		t.Fatal("final steering counts")
	}
	last := p.requests[1].Messages
	if last[len(last)-1].Role != "user" || last[len(last)-1].Text != "new direction" {
		t.Fatal("new direction not injected")
	}
}

func TestLoopDeliveryCommitBeforeCheckpointFailureIsRecoverable(t *testing.T) {
	s, r, h, a, p := loopFixture(t)
	mustSubmit(t, a, "one", "hello")
	if _, err := s.db.Exec(`CREATE TRIGGER fail_reply BEFORE INSERT ON event_outbox WHEN NEW.type='ai.erii.run.checkpointed.v1' AND EXISTS(SELECT 1 FROM main_reply) BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if err := a.Drain(context.Background()); !errors.Is(err, agent.ErrStorage) {
		t.Fatal("delivery checkpoint failure swallowed", err)
	}
	messages, err := h.List(context.Background(), idAgent(t, s))
	if err != nil || len(messages) != 2 {
		t.Fatal("Conversation did not commit first", messages, err)
	}
	if scalar(t, s, `SELECT count(*) FROM delivery_request`) != 1 || scalar(t, s, `SELECT count(*) FROM delivery_outbox`) != 1 {
		t.Fatal("delivery must commit before the reply checkpoint")
	}
	if scalar(t, s, `SELECT count(*) FROM run WHERE json_extract(checkpoint_json,'$.phase')='reply'`) != 1 || scalar(t, s, `SELECT count(*) FROM event_outbox WHERE type='ai.erii.delivery.pending.v1'`) != 1 {
		t.Fatal("delivery/event not committed before checkpoint")
	}
	if _, err = s.db.Exec(`DROP TRIGGER fail_reply`); err != nil {
		t.Fatal(err)
	}
	mustDrain(t, a)
	if err = r.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, err := h.List(context.Background(), idAgent(t, s))
	if err != nil || !reflect.DeepEqual(messages, after) || len(p.requests) != 2 || scalar(t, s, `SELECT count(*) FROM delivery_request`) != 1 {
		t.Fatal("recovery duplicated semantic/model result", after, err)
	}
}

func TestLoopToolResultsPersistImmediatelyAndInjectInDeclarationOrder(t *testing.T) {
	s, _, h, a, p := loopFixture(t)
	ctx := context.Background()
	slow, fast, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer close(release)
	registry, err := tool.NewRegistry([]tool.Definition{{Spec: tool.Spec{Name: "test__pure__echo", Version: 1, InputSchema: `{"type":"object"}`, ReplayPolicy: "safe", Permission: "read_only"}, Execute: func(ctx context.Context, args string) tool.Result {
		if args == `{"order":1}` {
			close(slow)
			<-release
			return tool.Result{Text: "first"}
		}
		close(fast)
		return tool.Result{Text: "second"}
	}}})
	if err != nil {
		t.Fatal(err)
	}
	r := s.Runs(h, registry)
	a.Store = r
	a.Registry = registry
	p.respond = func(request llm.GenerationRequest) (llm.GenerationResult, error) {
		last := request.Messages[len(request.Messages)-1]
		if last.Role == "tool" {
			return llm.GenerationResult{Text: "done"}, nil
		}
		return llm.GenerationResult{ToolCalls: []llm.ToolCall{{ID: "a", Name: "test__pure__echo", Arguments: `{"order":1}`}, {ID: "b", Name: "test__pure__echo", Arguments: `{"order":2}`}}}, nil
	}
	mustSubmit(t, a, "one", "run tools")
	done := make(chan error, 1)
	go func() { done <- a.Drain(ctx) }()
	<-slow
	<-fast
	// Wait on the durable fact, not the provider's self-report.
	deadline := time.Now().Add(3 * time.Second)
	for scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE provider_call_id='b' AND status='succeeded'`) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("fast completion blocked on slow call")
		}
		time.Sleep(time.Millisecond)
	}
	if scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE provider_call_id='a' AND status='running'`) != 1 {
		t.Fatal("slow invocation wrong state")
	}
	release <- struct{}{}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	last := p.requests[len(p.requests)-1].Messages
	got := []string{last[len(last)-2].ToolCallID, last[len(last)-1].ToolCallID, last[len(last)-2].Text, last[len(last)-1].Text}
	if !reflect.DeepEqual(got, []string{"a", "b", "first", "second"}) {
		t.Fatal("completion order leaked into model", got)
	}
}

func TestV6MainMigrationPreservesDataAndRollsBack(t *testing.T) {
	for _, broken := range []bool{false, true} {
		t.Run(fmt.Sprint(broken), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "state.db")
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			if err = migrate(context.Background(), db, migrations[:6]); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(`INSERT INTO agent_config VALUES(1,'prior-agent',NULL,'setup_required',1,11,11)`); err != nil {
				t.Fatal(err)
			}
			if broken {
				if _, err = db.Exec(`CREATE TABLE model_generation(obstruction TEXT)`); err != nil {
					t.Fatal(err)
				}
			}
			_ = db.Close()
			s, err := Open(context.Background(), root)
			if broken {
				if err == nil {
					s.Close()
					t.Fatal("migration ignored obstruction")
				}
				db, err = sql.Open("sqlite", path)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				var version, count int
				_ = db.QueryRow(`PRAGMA user_version`).Scan(&version)
				_ = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name IN ('run','capability_snapshot','main_input')`).Scan(&count)
				if version != 6 || count != 0 {
					t.Fatal("partial migration", version, count)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			d, err := s.Diagnostics(context.Background())
			if err != nil || d.AgentID != "prior-agent" || d.SchemaVersion != 17 {
				t.Fatal("prior identity changed", d, err)
			}
		})
	}
}

// Force shutdown between the scheduler's context check and its next database
// scan, while the already-owned generation reaches its durable boundary.
type shutdownScanStore struct {
	agent.ActorStore
	calls     int
	cancelled <-chan struct{}
	entering  chan struct{}
}

func (s *shutdownScanStore) Next(ctx context.Context) (string, error) {
	s.calls++
	if s.calls == 2 {
		close(s.entering)
		<-s.cancelled
	}
	return s.ActorStore.Next(ctx)
}
func TestLoopShutdownDuringSchedulerScanPreservesAcceptedGeneration(t *testing.T) {
	s, r, _, a, p := loopFixture(t)
	receipt := mustSubmit(t, a, "shutdown-scan", "hello")
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	cancelled := make(chan struct{})
	entering := make(chan struct{})
	original := p.respond
	p.respond = func(request llm.GenerationRequest) (llm.GenerationResult, error) {
		<-entering
		stop()
		close(cancelled)
		return original(request)
	}
	a.Store = &shutdownScanStore{ActorStore: r, cancelled: cancelled, entering: entering}
	if err := a.Drain(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("scheduler shutdown surfaced as infrastructure failure", err)
	}
	if scalar(t, s, `SELECT count(*) FROM run WHERE run_id=? AND status='running'`, receipt.RunID) != 1 || scalar(t, s, `SELECT count(*) FROM model_generation WHERE status='completed'`) != 1 {
		t.Fatal("shutdown lost accepted generation")
	}
	a.Store = r
	p.respond = original
	mustDrain(t, a)
	if len(p.requests) != 2 {
		t.Fatal("accepted generation repeated", len(p.requests))
	}
}

type shutdownClaimStore struct {
	agent.ActorStore
	stop context.CancelFunc
}

func (s shutdownClaimStore) Claim(ctx context.Context, id string) (agent.RunClaim, error) {
	s.stop()
	return s.ActorStore.Claim(ctx, id)
}
func TestLoopShutdownDuringSchedulerClaimDoesNotStartWork(t *testing.T) {
	s, r, _, a, _ := loopFixture(t)
	receipt := mustSubmit(t, a, "shutdown-claim", "hello")
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	a.Store = shutdownClaimStore{ActorStore: r, stop: stop}
	if err := a.Drain(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled claim surfaced as storage failure", err)
	}
	if scalar(t, s, `SELECT count(*) FROM run WHERE run_id=? AND status='queued'`, receipt.RunID) != 1 || scalar(t, s, `SELECT count(*) FROM model_generation`) != 0 {
		t.Fatal("cancelled scheduler started work")
	}
	a.Store = r
	mustDrain(t, a)
}

type acceptedShutdownClaimStore struct {
	agent.ActorStore
	stop context.CancelFunc
}

func (s acceptedShutdownClaimStore) Claim(ctx context.Context, id string) (agent.RunClaim, error) {
	c, err := s.ActorStore.Claim(ctx, id)
	s.stop()
	return c, err
}
func TestLoopShutdownImmediatelyAfterClaimReleasesOwnedWork(t *testing.T) {
	s, r, _, a, _ := loopFixture(t)
	receipt := mustSubmit(t, a, "shutdown-after-claim", "hello")
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	a.Store = acceptedShutdownClaimStore{ActorStore: r, stop: stop}
	if err := a.Drain(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("shutdown lost stable ownership boundary", err)
	}
	if scalar(t, s, `SELECT count(*) FROM run WHERE run_id=? AND status='running'`, receipt.RunID) != 1 || scalar(t, s, `SELECT count(*) FROM model_generation`) != 0 || scalar(t, s, `SELECT count(*) FROM run_claim WHERE token IS NOT NULL`) != 0 {
		t.Fatal("shutdown leaked owned work or started model")
	}
	a.Store = r
	mustDrain(t, a)
}
