package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/z-chenhao/erii/internal/agent"
	"github.com/z-chenhao/erii/internal/computer"
	"github.com/z-chenhao/erii/internal/llm"
	"github.com/z-chenhao/erii/internal/tool"
	"github.com/z-chenhao/erii/internal/tool/codemode"
)

func (r *Runs) ConfigureJavaScript(runtime codemode.JavaScriptRuntime) { r.javascript = runtime }
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

type codeBridge struct {
	t                 *tools
	req               codemode.ExecutionRequest
	attempt, exchange string
	registry          *tool.Registry
}
type nestedTools struct {
	*tools
	parent string
}

func (t nestedTools) Accept(ctx context.Context, exchange string, calls []llm.ToolCall, specs []tool.Spec) ([]tool.Invocation, error) {
	return t.tools.accept(ctx, exchange, calls, specs, t.parent)
}
func (t *tools) executeCode(ctx context.Context, inv tool.Invocation) (result tool.Result, executionErr error) {
	if t.runs.javascript == nil {
		return tool.Failure("PROVIDER_UNAVAILABLE"), nil
	}
	var args struct {
		Source string `json:"source"`
	}
	if json.Unmarshal([]byte(inv.Call.Arguments), &args) != nil {
		return tool.Failure("INVALID_ARGUMENT"), nil
	}
	b := &codeBridge{t: t}
	b.req = codemode.ExecutionRequest{ProtocolVersion: codemode.ProtocolVersion, RuntimeVersion: codemode.RuntimeVersion, PreludeVersion: codemode.PreludeVersion, ExecutionID: inv.ID, ParentInvocationID: inv.ID, RunID: t.claim.RunID(), Source: args.Source, SourceHash: digest(args.Source), Limits: codemode.DefaultLimits()}
	var fingerprintErr error
	b.req.RuntimeHash, b.req.PreludeHash, fingerprintErr = t.runs.javascript.Fingerprint()
	if fingerprintErr != nil {
		return tool.Result{}, agent.ErrStorage
	}
	defer func() {
		if b.req.ClaimToken != "" && b.req.ExecutionEpoch > 0 {
			result.HostFence = codeClaimFence{b.req.ClaimToken, b.req.ExecutionEpoch}
		}
	}()
	if err := b.claim(ctx, inv); err != nil {
		if errors.Is(err, tool.ErrReplay) {
			return tool.Failure("NON_DETERMINISTIC_REPLAY"), nil
		}
		return tool.Result{}, err
	}
	defer func() {
		if executionErr != nil {
			if err := b.abandon(); err != nil {
				executionErr = errors.Join(executionErr, err)
			}
		}
	}()
	var err error
	b.registry, err = t.runs.registry.Select(b.req.Tools)
	if err != nil {
		return tool.Result{}, err
	}
	inner := []tool.Spec{}
	for _, s := range b.req.Tools {
		if s.Name != codemode.ToolName {
			inner = append(inner, s)
		}
	}
	b.req.Tools = inner
	crashes := 0
	for {
		observed, err := b.wake(ctx)
		if err != nil {
			return tool.Result{}, err
		}
		var outcome codemode.Outcome
		var saved string
		err = t.runs.host.db.QueryRowContext(ctx, `SELECT outcome_json FROM code_outcome WHERE execution_id=?`, inv.ID).Scan(&saved)
		if err == nil {
			if json.Unmarshal([]byte(saved), &outcome) != nil {
				return tool.Result{}, agent.ErrStorage
			}
		} else if err == sql.ErrNoRows {
			b.attempt = newID()
			err = t.runs.write(ctx, func(tx *sql.Tx) error {
				if err := b.fence(ctx, tx); err != nil {
					return err
				}
				_, err := tx.ExecContext(ctx, `UPDATE code_execution_state SET replay_starts=replay_starts+1,version=version+1 WHERE execution_id=?`, inv.ID)
				return err
			})
			if err != nil {
				return tool.Result{}, err
			}
			vmCtx, stopVM := context.WithCancel(ctx)
			joined := make(chan struct{})
			go func() {
				select {
				case <-tool.ShutdownSignal(ctx):
					stopVM()
				case <-joined:
				}
			}()
			outcome, err = t.runs.javascript.Start(vmCtx, b.req, b)
			close(joined)
			stopVM()
			select {
			case <-tool.ShutdownSignal(ctx):
				return tool.Result{}, context.Canceled
			default:
			}
		}
		if err != nil {
			if errors.Is(err, codemode.ErrEventCommit) {
				return tool.Failure("PROVIDER_UNAVAILABLE"), nil
			}
			if ctx.Err() != nil {
				return tool.Result{}, ctx.Err()
			}
			// Transport death is infrastructure. Rebuild the VM from immutable source;
			// never turn a lost peer into a new logical nested call.
			crashes++
			if crashes <= 3 {
				continue
			}
			return tool.Result{}, agent.ErrStorage
		}
		if outcome.Kind == "completed" || outcome.Kind == "failed" {
			raw, _ := encoded(outcome)
			err = t.runs.write(ctx, func(tx *sql.Tx) error {
				if err := b.fence(ctx, tx); err != nil {
					return err
				}
				_, err := tx.ExecContext(ctx, `INSERT INTO code_outcome VALUES(?,?,?) ON CONFLICT DO NOTHING`, inv.ID, raw, time.Now().UnixMicro())
				return err
			})
			if err != nil {
				return tool.Result{}, err
			}
		}
		// All nonterminal Host calls are admitted before VM disposal. Execute them
		// through the ordinary pipeline with no retained JavaScript heap.
		err = b.drain(ctx)
		if err != nil && !errors.Is(err, tool.ErrSuspended) {
			return tool.Result{}, err
		}
		if errors.Is(err, tool.ErrSuspended) {
			t.runs.host.boundary("before_code_release")
			idle, e := b.release(ctx, observed)
			if e != nil {
				return tool.Result{}, e
			}
			if !idle {
				continue
			}
			return tool.Result{}, tool.ErrSuspended
		}
		switch outcome.Kind {
		case "suspended":
			continue
		case "cancelled":
			return tool.Failure("CANCELLED"), nil
		case "failed":
			if outcome.Error != nil {
				return tool.Normalize(tool.Result{Error: outcome.Error}), nil
			}
			return tool.Failure("INVALID_ARGUMENT"), nil
		case "completed":
			value, _ := json.Marshal(outcome.Output)
			return tool.Result{Text: string(value)}, nil
		}
		return tool.Failure("PROVIDER_UNAVAILABLE"), nil
	}
}

func (b *codeBridge) drain(ctx context.Context) error {
	calls := []llm.ToolCall{}
	rows, err := b.t.runs.host.db.QueryContext(ctx, `SELECT i.provider_call_id,i.tool_name,i.arguments_json FROM code_call c JOIN tool_invocation i USING(invocation_id) WHERE c.execution_id=? AND i.status NOT IN ('succeeded','failed','cancelled','unknown') ORDER BY i.created_at,c.call_id`, b.req.ExecutionID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var call llm.ToolCall
		if err = rows.Scan(&call.ID, &call.Name, &call.Arguments); err != nil {
			rows.Close()
			return err
		}
		calls = append(calls, call)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	suspended := false
	for start := 0; start < len(calls); start += 64 {
		end := start + 64
		if end > len(calls) {
			end = len(calls)
		}
		_, err := (tool.Executor{Registry: b.registry, Store: nestedTools{b.t.forCode(b), b.req.ExecutionID}, Concurrency: 4}).Execute(computer.WithExecution(ctx, b.req.ExecutionID), b.exchange, calls[start:end])
		if errors.Is(err, tool.ErrSuspended) {
			suspended = true
		} else if err != nil {
			return err
		}
	}
	if suspended {
		return tool.ErrSuspended
	}
	return nil
}

func (b *codeBridge) Handle(ctx context.Context, method string, m codemode.Message) (codemode.Reply, error) {
	empty := codemode.Reply{}
	if m.ClaimToken != b.req.ClaimToken || m.ExecutionEpoch != b.req.ExecutionEpoch || m.ExecutionID != b.req.ExecutionID || m.ParentInvocationID != b.req.ParentInvocationID || m.RunID != b.req.RunID {
		return empty, agent.ErrConflict
	}
	// Persist every incoming message, including frozen Host parameters, before ack.
	err := b.t.runs.write(ctx, func(tx *sql.Tx) error {
		if err := b.fence(ctx, tx); err != nil {
			return err
		}
		value, _ := encoded(m)
		var old string
		err := tx.QueryRowContext(ctx, `SELECT message_json FROM code_event WHERE execution_id=? AND attempt_id=? AND sequence=?`, m.ExecutionID, b.attempt, m.Sequence).Scan(&old)
		if err == nil {
			if old != value {
				return tool.ErrReplay
			}
			return nil
		}
		if err != sql.ErrNoRows {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO code_event VALUES(?,?,?,?)`, m.ExecutionID, b.attempt, m.Sequence, value); err != nil {
			return err
		}
		if method == "execution.event" {
			if (m.Kind == "execution_completed" || m.Kind == "execution_failed") && m.Outcome != nil {
				outcome, _ := encoded(m.Outcome)
				if _, err := tx.ExecContext(ctx, `INSERT INTO code_outcome VALUES(?,?,?) ON CONFLICT DO NOTHING`, m.ExecutionID, outcome, time.Now().UnixMicro()); err != nil {
					return err
				}
			}
			return runEvent(ctx, tx, b.t.claim.RunID(), "codemode."+m.Kind, 1)
		}
		return nil
	})
	if err != nil {
		return empty, err
	}
	switch method {
	case "execution.event":
		// Opt-in exact recovery checkpoint; no production query or hidden fixture.
		if m.Kind == "execution_suspended" && b.t.runs.host.MainBoundary != nil {
			var completed, active int
			err := b.t.runs.host.db.QueryRowContext(ctx, `SELECT coalesce(sum(child.status='completed'),0),coalesce(sum(child.status IN ('queued','running','waiting')),0) FROM code_call c JOIN agent_invocation a USING(invocation_id) JOIN run child ON child.run_id=a.child_run_id WHERE c.execution_id=?`, b.req.ExecutionID).Scan(&completed, &active)
			if err != nil {
				return empty, err
			}
			if completed == 37 && active == 8 {
				b.t.runs.host.boundary("code_37_completed_8_active")
			}
		}
		return empty, nil
	case "host.invoke":
		n, err := strconv.Atoi(m.CallID)
		if err != nil || n < 1 || n > 10000 || len(m.CallKey) != 64 {
			return denied("PERMISSION_DENIED"), nil
		}
		if m.ToolName == codemode.ToolName {
			return denied("PERMISSION_DENIED"), nil
		}
		visible := false
		for _, s := range b.req.Tools {
			visible = visible || s.Name == m.ToolName
		}
		if !visible {
			return denied("PERMISSION_DENIED"), nil
		}
		// IDs are generated in trusted Runtime code; script arguments cannot select them.
		arguments, err := llm.CanonicalJSON(m.Arguments)
		if err != nil {
			return denied("INVALID_ARGUMENT"), nil
		}
		call := llm.ToolCall{ID: b.req.ExecutionID + "/" + m.CallKey, Name: m.ToolName, Arguments: string(arguments)}
		invocations, err := (nestedTools{b.t.forCode(b), b.req.ExecutionID}).Accept(ctx, b.exchange, []llm.ToolCall{call}, b.registry.Describe())
		if errors.Is(err, tool.ErrReplay) {
			return denied("NON_DETERMINISTIC_REPLAY"), nil
		}
		if err != nil {
			return empty, err
		}
		accepted := invocations[0]
		if accepted.Status != "succeeded" && accepted.Status != "failed" && accepted.Status != "cancelled" && accepted.Status != "unknown" {
			return codemode.Reply{Suspended: true}, nil
		}
		err = b.t.runs.write(ctx, func(tx *sql.Tx) error {
			if err := b.fence(ctx, tx); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `UPDATE code_execution_state SET replay_hits=replay_hits+1,version=version+1 WHERE execution_id=?`, b.req.ExecutionID)
			return err
		})
		if err != nil {
			return empty, err
		}
		r := accepted.Result
		if accepted.Status == "unknown" {
			r = tool.Failure("UNKNOWN_SIDE_EFFECT")
		}
		if r.Error != nil {
			return b.deliver(ctx, call.ID, codemode.Reply{Error: r.Error})
		}
		// Providers normalize their business result to JSON text at ToolExecutor.
		var value any
		if json.Unmarshal([]byte(r.Text), &value) != nil {
			value = r.Text
		}
		if len(r.ArtifactRefs) > 0 {
			value = map[string]any{"text": r.Text, "artifactRefs": r.ArtifactRefs}
		}
		return b.deliver(ctx, call.ID, codemode.Reply{Value: value})
	case "host.store_get", "host.store_put":
		return b.store(ctx, method, m.CallKey, m.Arguments)
	case "host.discover":
		return b.discover(m), nil
	}
	return denied("PERMISSION_DENIED"), nil
}
func denied(code string) codemode.Reply { return codemode.Reply{Error: tool.Failure(code).Error} }
func (b *codeBridge) store(ctx context.Context, method, callKey string, raw json.RawMessage) (codemode.Reply, error) {
	var args struct {
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
	}
	if len(callKey) != 64 || json.Unmarshal(raw, &args) != nil || len(args.Key) == 0 || len(args.Key) > 256 {
		return denied("INVALID_ARGUMENT"), nil
	}
	reply := codemode.Reply{}
	err := b.t.runs.write(ctx, func(tx *sql.Tx) error {
		if err := b.fence(ctx, tx); err != nil {
			return err
		}
		requestHash := digest(method + ":" + string(raw))
		var oldHash, oldReply string
		err := tx.QueryRowContext(ctx, `SELECT request_hash,reply_json FROM code_store_call WHERE execution_id=? AND call_key=?`, b.req.ExecutionID, callKey).Scan(&oldHash, &oldReply)
		if err == nil {
			if oldHash != requestHash {
				reply = denied("NON_DETERMINISTIC_REPLAY")
				return nil
			}
			return json.Unmarshal([]byte(oldReply), &reply)
		}
		if err != sql.ErrNoRows {
			return err
		}
		save := func() error {
			value, _ := encoded(reply)
			_, err := tx.ExecContext(ctx, `INSERT INTO code_store_call VALUES(?,?,?,?)`, b.req.ExecutionID, callKey, requestHash, value)
			return err
		}
		if method == "host.store_put" {
			if !json.Valid(args.Value) || len(args.Value) > 262144 || strings.Contains(string(args.Value), `"handle_token"`) || strings.Contains(string(args.Value), `"handleToken"`) {
				reply = denied("INVALID_ARGUMENT")
				return nil
			}
			var size int
			if err := tx.QueryRowContext(ctx, `SELECT coalesce(sum(length(CAST(value_json AS BLOB))+length(key)),0) FROM code_store WHERE execution_id=? AND key!=?`, b.req.ExecutionID, args.Key).Scan(&size); err != nil {
				return err
			}
			if size+len(args.Value)+len(args.Key) > 1<<20 {
				reply = denied("RESOURCE_LIMIT")
				return nil
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO code_store(execution_id,key,value_json,version,value_hash,updated_at) VALUES(?,?,?,1,?,?) ON CONFLICT(execution_id,key) DO UPDATE SET value_json=excluded.value_json,value_hash=excluded.value_hash,updated_at=excluded.updated_at,version=code_store.version+1`, b.req.ExecutionID, args.Key, string(args.Value), digest(string(args.Value)), time.Now().UnixMicro())
			if err != nil {
				return err
			}
			return save()
		}
		var value, hash string
		err = tx.QueryRowContext(ctx, `SELECT value_json,value_hash FROM code_store WHERE execution_id=? AND key=?`, b.req.ExecutionID, args.Key).Scan(&value, &hash)
		if err == sql.ErrNoRows {
			return save()
		}
		if err != nil {
			return err
		}
		if hash != digest(value) {
			return tool.ErrReplay
		}
		reply.Value = json.RawMessage(value)
		return save()
	})
	return reply, err
}
func (b *codeBridge) discover(m codemode.Message) codemode.Reply {
	type description struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		InputSchema string `json:"inputSchema,omitempty"`
	}
	switch m.ToolName {
	case "describeTool":
		var name string
		if json.Unmarshal(m.Arguments, &name) != nil {
			return denied("INVALID_ARGUMENT")
		}
		for _, s := range b.req.Tools {
			if codemode.ComputerBridge(s.Name) {
				continue
			}
			if s.Name == name {
				return codemode.Reply{Value: codemode.Describe(s)}
			}
		}
	case "describeNamespace":
		var name string
		if json.Unmarshal(m.Arguments, &name) != nil {
			return denied("INVALID_ARGUMENT")
		}
		if name == "cua" {
			return codemode.Reply{Value: map[string]any{"name": "cua", "description": "Discover inventory with getState/listApps/listWindows/listBrowsers/listTabs. Bind with getApp/getBrowser/getTab/createBrowserTab. Bound surfaces expose documentation, observe and capabilities. Re-observe after every action. Handles expire with the cell."}}
		}
		if name == "agent" {
			for _, s := range b.req.Tools {
				if codemode.ComputerBridge(s.Name) {
					continue
				}
				if s.Name == agent.RunToolName {
					return codemode.Reply{Value: map[string]any{"name": "agent", "description": "run({task, artifactRefs?}) returns Promise<{text, artifactRefs}>. Creates one general Child Worker; does not create a Task. Host-accepted calls continue after script end; errors reject normally."}}
				}
			}
		}
		items := []description{}
		for _, s := range b.req.Tools {
			if codemode.ComputerBridge(s.Name) {
				continue
			}
			if codemode.NamespaceMatches(s.Name, name) {
				items = append(items, description{s.Name, s.Description, ""})
			}
		}
		if len(items) > 0 {
			return codemode.Reply{Value: map[string]any{"name": name, "tools": items}}
		}
	case "searchTools":
		var args []json.RawMessage
		if json.Unmarshal(m.Arguments, &args) != nil || len(args) == 0 {
			return denied("INVALID_ARGUMENT")
		}
		var query string
		if json.Unmarshal(args[0], &query) != nil {
			return denied("INVALID_ARGUMENT")
		}
		options := struct {
			Limit     int    `json:"limit"`
			Namespace string `json:"namespace"`
		}{Limit: 20}
		if len(args) > 1 && json.Unmarshal(args[1], &options) != nil {
			return denied("INVALID_ARGUMENT")
		}
		if options.Limit < 1 || options.Limit > 100 {
			return denied("INVALID_ARGUMENT")
		}
		items := []description{}
		for _, s := range b.req.Tools {
			if codemode.ComputerBridge(s.Name) {
				continue
			}
			if strings.Contains(strings.ToLower(s.Name+" "+s.Description), strings.ToLower(query)) && (options.Namespace == "" || codemode.NamespaceMatches(s.Name, options.Namespace)) {
				items = append(items, description{s.Name, s.Description, ""})
				if len(items) >= options.Limit {
					break
				}
			}
		}
		return codemode.Reply{Value: items}
	}
	return codemode.Reply{}
}
