package codemode

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

var ErrEventCommit = errors.New("JavaScript event commit rejected")

var ErrProtocol = errors.New("JavaScript runtime protocol failure")

type Sidecar struct {
	Command, Script string
	mu              sync.Mutex
	active          map[string]*process
}
type process struct {
	cmd   *exec.Cmd
	input io.WriteCloser
	write sync.Mutex
}
type rpc struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func (p *process) send(value any) error {
	p.write.Lock()
	defer p.write.Unlock()
	b, err := json.Marshal(value)
	if err != nil || len(b) > 1<<20 {
		return ErrProtocol
	}
	_, err = p.input.Write(append(b, '\n'))
	return err
}
func (s *Sidecar) Start(ctx context.Context, req ExecutionRequest, sink ExecutionEventSink) (Outcome, error) {
	cmd := exec.Command(s.Command, s.Script)
	// Never inherit NODE_OPTIONS, credentials, account environment or user paths.
	cmd.Env = []string{"PATH=/usr/bin:/bin", "TZ=UTC", "LANG=C.UTF-8"}
	input, err := cmd.StdinPipe()
	if err != nil {
		return Outcome{}, ErrProtocol
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return Outcome{}, ErrProtocol
	}
	p := &process{cmd: cmd, input: input}
	s.mu.Lock()
	if s.active == nil {
		s.active = map[string]*process{}
	}
	if _, exists := s.active[req.ExecutionID]; exists {
		s.mu.Unlock()
		return Outcome{}, ErrProtocol
	}
	s.active[req.ExecutionID] = p
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.active, req.ExecutionID); s.mu.Unlock() }()
	if err = cmd.Start(); err != nil {
		return Outcome{}, ErrProtocol
	}
	done := make(chan struct{})
	defer close(done)
	defer func() { _ = input.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	go func() {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
		case <-done:
		}
	}()
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), (1<<20)+1)
	initialize := map[string]any{"jsonrpc": "2.0", "id": "init", "method": "runtime.initialize", "params": map[string]string{"protocolVersion": ProtocolVersion, "runtimeVersion": RuntimeVersion, "preludeVersion": PreludeVersion}}
	if p.send(initialize) != nil || !scanner.Scan() {
		return Outcome{}, ErrProtocol
	}
	var msg rpc
	if json.Unmarshal(scanner.Bytes(), &msg) != nil || string(msg.ID) != `"init"` || len(msg.Error) > 0 {
		return Outcome{}, ErrProtocol
	}
	var identity struct {
		Protocol    string `json:"protocolVersion"`
		Runtime     string `json:"runtimeVersion"`
		Prelude     string `json:"preludeVersion"`
		RuntimeHash string `json:"runtimeHash"`
		PreludeHash string `json:"preludeHash"`
	}
	if json.Unmarshal(msg.Result, &identity) != nil || identity.Protocol != ProtocolVersion || identity.Runtime != RuntimeVersion || identity.Prelude != PreludeVersion {
		return Outcome{}, ErrProtocol
	}
	if (req.RuntimeHash != "" && req.RuntimeHash != identity.RuntimeHash) || (req.PreludeHash != "" && req.PreludeHash != identity.PreludeHash) {
		return Outcome{}, ErrProtocol
	}
	req.RuntimeHash = identity.RuntimeHash
	req.PreludeHash = identity.PreludeHash
	if p.send(map[string]any{"jsonrpc": "2.0", "id": "start", "method": "execution.start", "params": req}) != nil {
		return Outcome{}, ErrProtocol
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	// Calls can execute concurrently; events and Store stay ordered on this reader.
	var sequence int64
	var terminal *Outcome
	received := map[int64]string{}
	for scanner.Scan() {
		var msg rpc
		if json.Unmarshal(scanner.Bytes(), &msg) != nil || msg.JSONRPC != "2.0" {
			return Outcome{}, ErrProtocol
		}
		if msg.Method == "" {
			if string(msg.ID) != `"start"` || len(msg.Error) > 0 {
				return Outcome{}, ErrProtocol
			}
			var outcome Outcome
			if json.Unmarshal(msg.Result, &outcome) != nil {
				return Outcome{}, ErrProtocol
			}
			if terminal == nil {
				return Outcome{}, ErrProtocol
			}
			a, _ := json.Marshal(outcome)
			b, _ := json.Marshal(terminal)
			if !bytes.Equal(a, b) {
				return Outcome{}, ErrProtocol
			}
			switch outcome.Kind {
			case "completed", "suspended", "failed", "cancelled":
				return outcome, nil
			}
			return Outcome{}, ErrProtocol
		}
		var event Message
		if json.Unmarshal(msg.Params, &event) != nil || event.ClaimToken != req.ClaimToken || event.ExecutionEpoch != req.ExecutionEpoch || event.ProtocolVersion != ProtocolVersion || event.RuntimeID != RuntimeVersion || event.ExecutionID != req.ExecutionID || event.ParentInvocationID != req.ParentInvocationID || event.RunID != req.RunID || event.Sequence < 1 {
			return Outcome{}, ErrProtocol
		}
		value := string(msg.Params)
		duplicate := false
		if previous, exists := received[event.Sequence]; exists {
			duplicate = true
			if previous != value || msg.Method != "execution.event" {
				return Outcome{}, ErrProtocol
			}
		} else {
			if event.Sequence != sequence+1 {
				return Outcome{}, ErrProtocol
			}
			sequence = event.Sequence
			received[event.Sequence] = value
		}
		if _, err := time.Parse(time.RFC3339Nano, event.OccurredAt); err != nil {
			return Outcome{}, ErrProtocol
		}
		if !duplicate && (event.Kind == "execution_completed" || event.Kind == "execution_failed" || event.Kind == "execution_cancelled" || event.Kind == "execution_suspended") {
			if event.Outcome == nil || terminal != nil {
				return Outcome{}, ErrProtocol
			}
			terminal = event.Outcome
		}
		respond := func(msg rpc, event Message) error {
			reply, err := sink.Handle(ctx, msg.Method, event)
			response := map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": reply}
			if err != nil {
				delete(response, "result")
				response["error"] = map[string]any{"code": -32000, "message": "PROVIDER_UNAVAILABLE"}
			}
			_ = p.send(response)
			return err
		}
		switch msg.Method {
		case "host.invoke":
			wg.Add(1)
			go func(m rpc, e Message) { defer wg.Done(); _ = respond(m, e) }(msg, event)
		case "execution.event", "host.discover", "host.store_get", "host.store_put":
			err := respond(msg, event)
			if msg.Method == "execution.event" && err != nil {
				return Outcome{}, ErrEventCommit
			}
		default:
			return Outcome{}, ErrProtocol
		}
	}
	if ctx.Err() != nil {
		return Outcome{}, ctx.Err()
	}
	return Outcome{}, ErrProtocol
}
func (s *Sidecar) Cancel(ctx context.Context, id, reason string) error {
	s.mu.Lock()
	p := s.active[id]
	s.mu.Unlock()
	if p == nil {
		return nil
	}
	return p.send(map[string]any{"jsonrpc": "2.0", "id": "cancel", "method": "execution.cancel", "params": map[string]string{"executionId": id, "reason": reason}})
}
func (s *Sidecar) Dispose(ctx context.Context, id string) error { return s.Cancel(ctx, id, "dispose") }

// Fingerprint covers executable sources and the pinned engine bytes. A version
// string alone cannot prove an installed Runtime is the one a Run froze.
func (s *Sidecar) Fingerprint() (string, string, error) {
	root := filepath.Dir(s.Script)
	hashFiles := func(paths []string) (string, error) {
		h := sha256.New()
		for _, path := range paths {
			b, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				return "", ErrProtocol
			}
			_, _ = h.Write(b)
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	runtime, err := hashFiles([]string{"sidecar.mjs", "worker.mjs", "vendor/vm.mjs", "../package-lock.json", "../node_modules/quickjs-wasi/quickjs.wasm"})
	if err != nil {
		return "", "", err
	}
	prelude, err := hashFiles([]string{"prelude.mjs", "vendor/prelude.mjs"})
	return runtime, prelude, err
}
