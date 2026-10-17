package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/z-chenhao/erii/internal/llm"
	"github.com/z-chenhao/erii/internal/tool"
	"github.com/z-chenhao/erii/internal/tool/codemode"
)

func realCodeRuntime(t *testing.T) *codemode.Sidecar {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node is required for Sidecar contract tests")
	}
	script, err := filepath.Abs("../../../runtime/javascript/src/sidecar.mjs")
	if err != nil {
		t.Fatal(err)
	}
	return &codemode.Sidecar{Command: node, Script: script}
}

// Hold execution slots through the real store, without timing-dependent sleeps.
// Wrappers must start even when a Tool owns the only slot, and must not prevent
// the next Tool from acquiring that slot once it is released.
func TestCodeModeWrapperToolCapacityOwnership(t *testing.T) {
	for _, scope := range []string{"global", "agent", "run", "provider", "tool", "conflict"} {
		t.Run(scope, func(t *testing.T) {
			ctx := context.Background()
			echo, _ := tool.EchoRegistry().Resolve("builtin__text__echo")
			limits := ToolLimits{Global: 32, Agent: 32, Run: 32, Provider: 32, Tool: 32}
			switch scope {
			case "global":
				limits.Global = 1
			case "agent":
				limits.Agent = 1
			case "run":
				limits.Run = 1
			case "provider":
				limits.Provider = 1
			case "tool":
				limits.Tool = 1
			case "conflict":
				echo.Spec.ConflictPolicy = "same-target-v1"
				echo.ConflictKeys = func(string) []string { return []string{"same-resource"} }
			}
			calls := []llm.ToolCall{
				{ID: "first", Name: echo.Spec.Name, Arguments: `{"text":"first"}`},
				{ID: "second", Name: echo.Spec.Name, Arguments: `{"text":"second"}`},
				{ID: "wrapper-one", Name: codemode.ToolName, Arguments: `{"source":"return 1;"}`},
				{ID: "wrapper-two", Name: codemode.ToolName, Arguments: `{"source":"return 2;"}`},
			}
			s, r, _, e, exchange := admitPolicy(t, []tool.Definition{codemode.Definition(), echo}, calls)
			r.ConfigureToolLimits(limits)
			invs, err := e.Store.Accept(ctx, exchange, calls, e.Registry.Describe())
			if err != nil {
				t.Fatal(err)
			}
			for _, index := range []int{0, 2, 3} {
				if err = e.Store.Start(ctx, invs[index].ID); err != nil {
					t.Fatalf("start %s with one actual Tool running: %v", calls[index].ID, err)
				}
			}
			if err = e.Store.Start(ctx, invs[1].ID); !errors.Is(err, tool.ErrResource) {
				t.Fatalf("competing actual Tool bypassed %s limit: %v", scope, err)
			}
			if err = e.Store.Finish(ctx, invs[0].ID, tool.Result{Text: "first"}); err != nil {
				t.Fatal(err)
			}
			if err = e.Store.Start(ctx, invs[1].ID); err != nil {
				t.Fatalf("wrappers retained actual Tool capacity: %v", err)
			}
			if got := scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE tool_name=? AND status='running'`, echo.Spec.Name); got != 1 {
				t.Fatalf("running actual Tools = %d", got)
			}
		})
	}
}

func TestCodeModeRealRuntimeNestedToolAndStore(t *testing.T) {
	s, r, a, p := childFixture(t, codemode.Definition())
	r.ConfigureJavaScript(realCodeRuntime(t))
	r.ConfigureToolLimits(ToolLimits{Global: 1, Agent: 1, Run: 1, Provider: 1, Tool: 1})
	p.respond = func(req llm.GenerationRequest) (llm.GenerationResult, error) {
		if req.Messages[len(req.Messages)-1].Role == "tool" {
			return llm.GenerationResult{Text: "observed"}, nil
		}
		source := `store("persisted", {n: 5}); const x = await tools.builtin__text__echo({text:"nested"}); return [x, load("persisted")];`
		args, _ := json.Marshal(map[string]string{"source": source})
		return calls(codemode.ToolName, string(args)), nil
	}
	mustSubmit(t, a, "code", "execute")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := a.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM code_execution`:                                                             1,
		`SELECT count(*) FROM code_call`:                                                                  1,
		`SELECT count(*) FROM code_store WHERE json_extract(value_json,'$.n')=5`:                          1,
		`SELECT count(*) FROM tool_invocation WHERE status='succeeded' AND attempt_count=1`:               2,
		`SELECT count(*) FROM tool_invocation WHERE parent_invocation_id IS NOT NULL`:                     1,
		`SELECT count(*) FROM code_event WHERE json_extract(message_json,'$.kind')='execution_completed'`: 1,
	} {
		if got := scalar(t, s, query); got != want {
			t.Fatalf("%s = %d, want %d", query, got, want)
		}
	}
	var result string
	if err := s.db.QueryRow(`SELECT result_json FROM tool_invocation WHERE tool_name=?`, codemode.ToolName).Scan(&result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `nested`) || !strings.Contains(result, `return`) {
		t.Fatal(result)
	}
}
func TestCodeModeMigrateEmptyAndV15(t *testing.T) {
	for _, prior := range []int{0, 15} {
		t.Run(strconv.Itoa(prior), func(t *testing.T) {
			ctx := context.Background()
			db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "state.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if prior > 0 {
				if err = migrate(ctx, db, migrations[:prior]); err != nil {
					t.Fatal(err)
				}
			}
			broken := append([]string{}, migrations...)
			broken[len(broken)-1] += "\nINVALID SQL;"
			if err = migrate(ctx, db, broken); err == nil {
				t.Fatal("accepted broken migration")
			}
			var version int
			db.QueryRow(`PRAGMA user_version`).Scan(&version)
			want := prior
			if version != want {
				t.Fatalf("failed migration version %d", version)
			}
			if err = migrate(ctx, db, migrations); err != nil {
				t.Fatal(err)
			}
			if err = migrate(ctx, db, migrations); err != nil {
				t.Fatal(err)
			}
			var tables int
			db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('code_execution','code_call','code_store','code_event','code_outcome')`).Scan(&tables)
			if tables != 5 {
				t.Fatal(tables)
			}
		})
	}
}
func TestNestedAdmissionRejectsUnacceptedParent(t *testing.T) {
	_, r, a, _ := childFixture(t, codemode.Definition())
	receipt := mustSubmit(t, a, "parent-guard", "hello")
	claim, err := r.Claim(context.Background(), receipt.RunID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = (nestedTools{&tools{runs: r, claim: claim}, "forged-parent"}).Accept(context.Background(), "forged-exchange", []llm.ToolCall{{ID: "1", Name: "builtin__text__echo", Arguments: `{"text":"x"}`}}, a.Registry.Describe())
	if err == nil {
		t.Fatal("accepted forged parent")
	}
}

var _ tool.Store = nestedTools{}

func TestCodeModeStoreAndEventCommitFailuresDoNotLeakSuccess(t *testing.T) {
	for _, boundary := range []string{"store", "terminal-event"} {
		t.Run(boundary, func(t *testing.T) {
			s, r, a, p := childFixture(t, codemode.Definition())
			r.ConfigureJavaScript(realCodeRuntime(t))
			trigger := `CREATE TRIGGER reject_store BEFORE INSERT ON code_store BEGIN SELECT RAISE(ABORT,'injected store failure'); END`
			source := `store("x",1); return "UNCOMMITTED_SUCCESS";`
			if boundary == "terminal-event" {
				trigger = `CREATE TRIGGER reject_code_event BEFORE INSERT ON code_event WHEN json_extract(NEW.message_json,'$.kind')='execution_completed' BEGIN SELECT RAISE(ABORT,'injected event failure'); END`
				source = `return "UNCOMMITTED_SUCCESS";`
			}
			if _, err := s.db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			p.respond = func(req llm.GenerationRequest) (llm.GenerationResult, error) {
				if req.Messages[len(req.Messages)-1].Role == "tool" {
					return llm.GenerationResult{Text: "observed error"}, nil
				}
				args, _ := json.Marshal(map[string]string{"source": source})
				return calls(codemode.ToolName, string(args)), nil
			}
			mustSubmit(t, a, boundary, "execute")
			mustDrain(t, a)
			if scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE tool_name='builtin__codemode__execute' AND status='failed'`) != 1 {
				t.Fatal("reported success before commit")
			}
			if scalar(t, s, `SELECT count(*) FROM code_store`) != 0 {
				t.Fatal("failed Store persisted")
			}
			var raw string
			if err := s.db.QueryRow(`SELECT result_json FROM tool_invocation WHERE tool_name=?`, codemode.ToolName).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(raw, "UNCOMMITTED_SUCCESS") || strings.Contains(raw, "injected") {
				t.Fatal("uncommitted data or internal error exposed", raw)
			}
		})
	}
}
