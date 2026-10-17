package codemode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/z-chenhao/erii/internal/tool"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type sinkFunc func(context.Context, string, Message) (Reply, error)

func (f sinkFunc) Handle(ctx context.Context, method string, m Message) (Reply, error) {
	return f(ctx, method, m)
}
func requestFixture(source string) ExecutionRequest {
	hash := sha256.Sum256([]byte(source))
	return ExecutionRequest{ProtocolVersion: ProtocolVersion, RuntimeVersion: RuntimeVersion, PreludeVersion: PreludeVersion, ExecutionID: "execution", ClaimToken: "claim", ExecutionEpoch: 1, ProviderCallID: "provider-call", ParentInvocationID: "parent", RunID: "run", Kind: "main", Tools: []tool.Spec{}, Source: source, SourceHash: hex.EncodeToString(hash[:]), Limits: DefaultLimits()}
}
func TestRealSidecarProtocolAndExplicitOutput(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("../../../runtime/javascript/src/sidecar.mjs")
	if err != nil {
		t.Fatal(err)
	}
	runtime := &Sidecar{Command: node, Script: script}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events := []string{}
	result, err := runtime.Start(ctx, requestFixture(`console.log("hidden");text("chosen"); return 12;`), sinkFunc(func(ctx context.Context, method string, m Message) (Reply, error) {
		events = append(events, m.Kind)
		return Reply{}, nil
	}))
	if err != nil || result.Kind != "completed" || len(result.Output) != 2 {
		t.Fatalf("%+v %v", result, err)
	}
	if len(events) != 3 || events[0] != "execution_started" || events[2] != "execution_completed" {
		t.Fatal(events)
	}
}
func TestSidecarRejectsForgedProtocolAndDeduplicatesEvents(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"forged-run", "forged-method", "completion-without-event", "completion-drift", "duplicate-event", "uncommitted-terminal", "event-gap"} {
		t.Run(mode, func(t *testing.T) {
			// This is a malicious transport peer fixture, never a JS interpreter.
			source := `import {createInterface} from 'node:readline';
const send = x => process.stdout.write(JSON.stringify({jsonrpc:'2.0',...x})+'\n');
let request;
createInterface({input:process.stdin}).on('line',line=>{
 const m=JSON.parse(line);
 if(m.method==='runtime.initialize')send({id:m.id,result:m.params});
 if(m.method==='execution.start'){
  request=m.params;
  const event={protocolVersion:'1',runtimeId:'erii-js-v2',executionId:request.executionId,parentInvocationId:request.parentInvocationId,runId:request.runId,claimToken:request.claimToken,executionEpoch:request.executionEpoch,sequence:1,occurredAt:'2026-10-17T00:00:00Z',kind:'execution_completed',outcome:{kind:'completed',output:[],pendingCallIds:[]}};
  if(MODE==='completion-without-event'){send({id:'start',result:event.outcome});return;}
  if(MODE==='forged-run') event.runId='forged';
  if(MODE==='event-gap') event.sequence=2;
  send({id:'event',method:MODE==='forged-method'?'host.rawDriver':'execution.event',params:event});
  if(MODE==='duplicate-event')send({id:'duplicate',method:'execution.event',params:event});
 }
 if(m.id==='event' && MODE!=='duplicate-event')send({id:'start',result:{kind:MODE==='completion-drift'?'failed':'completed',output:[],pendingCallIds:[]}});
 if(m.id==='duplicate')send({id:'start',result:{kind:'completed',output:[],pendingCallIds:[]}});
});`
			source = "const MODE=" + `"` + mode + `"` + ";\n" + source
			path := filepath.Join(t.TempDir(), "peer.mjs")
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			runtime := &Sidecar{Command: node, Script: path}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			seen := map[int64]bool{}
			commits := 0
			_, err := runtime.Start(ctx, requestFixture("return 1;"), sinkFunc(func(ctx context.Context, method string, m Message) (Reply, error) {
				if mode == "uncommitted-terminal" {
					return Reply{}, ErrProtocol
				}
				if !seen[m.Sequence] {
					seen[m.Sequence] = true
					commits++
				}
				return Reply{}, nil
			}))
			if mode == "duplicate-event" {
				if err != nil || commits != 1 {
					t.Fatalf("dedup: %d %v", commits, err)
				}
			} else if err == nil {
				t.Fatal("accepted forged transport")
			}
		})
	}
}
func TestSidecarCancelStopsCPUExecution(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	script, _ := filepath.Abs("../../../runtime/javascript/src/sidecar.mjs")
	runtime := &Sidecar{Command: node, Script: script}
	ready := make(chan struct{})
	var once sync.Once
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan Outcome, 1)
	req := requestFixture("while(true){}")
	req.Limits.InterruptBudget = 100000
	go func() {
		out, err := runtime.Start(ctx, req, sinkFunc(func(ctx context.Context, method string, m Message) (Reply, error) {
			if m.Kind == "execution_started" {
				once.Do(func() { close(ready) })
			}
			return Reply{}, nil
		}))
		if err != nil {
			out.Kind = "error"
		}
		done <- out
	}()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("runtime not ready")
	}
	if err = runtime.Cancel(ctx, req.ExecutionID, "test"); err != nil {
		t.Fatal(err)
	}
	select {
	case out := <-done:
		if out.Kind != "cancelled" {
			raw, _ := json.Marshal(out)
			t.Fatal(string(raw))
		}
	case <-ctx.Done():
		t.Fatal("cancel did not reap runtime")
	}
}
