package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"

	"github.com/z-chenhao/erii/internal/agent"
	"github.com/z-chenhao/erii/internal/llm"
	"github.com/z-chenhao/erii/internal/tool"
	"github.com/z-chenhao/erii/internal/tool/codemode"
)

type tools struct {
	runs      *Runs
	claim     agent.RunClaim
	codeOwner *codeBridge
}

func (t *tools) forCode(b *codeBridge) *tools {
	return &tools{runs: t.runs, claim: t.claim, codeOwner: b}
}
func (t *tools) fence(ctx context.Context, tx *sql.Tx) error {
	if t.codeOwner != nil {
		return t.codeOwner.fence(ctx, tx)
	}
	return fence(ctx, tx, t.claim)
}

func (r *Runs) ToolStore(c agent.RunClaim) tool.Store { return &tools{runs: r, claim: c} }

func (t *tools) Accept(ctx context.Context, exchange string, calls []llm.ToolCall, specs []tool.Spec) ([]tool.Invocation, error) {
	return t.accept(ctx, exchange, calls, specs, "")
}

func (t *tools) accept(ctx context.Context, exchange string, calls []llm.ToolCall, specs []tool.Spec, parent string) ([]tool.Invocation, error) {
	out := []tool.Invocation{}
	err := t.runs.write(ctx, func(tx *sql.Tx) error {
		out = out[:0]
		if err := t.fence(ctx, tx); err != nil {
			return err
		}
		var frozen string
		if err := tx.QueryRowContext(ctx, `SELECT c.tools_json FROM run r JOIN capability_snapshot c ON c.snapshot_id=r.capability_snapshot_id WHERE r.run_id=?`, t.claim.RunID()).Scan(&frozen); err != nil {
			return err
		}
		current, _ := encoded(specs)
		if frozen != current {
			if parent != "" {
				return tool.ErrReplay
			}
			return llm.ModelIncompatible
		}
		if parent == "" {
			// Only calls in a persisted complete model response can be accepted.
			var value string
			if err := tx.QueryRowContext(ctx, `SELECT result_json FROM model_generation WHERE generation_id=? AND run_id=? AND status='completed'`, exchange, t.claim.RunID()).Scan(&value); err != nil {
				return err
			}
			var response llm.GenerationResult
			if json.Unmarshal([]byte(value), &response) != nil {
				return agent.ErrStorage
			}
			a, _ := encoded(response.ToolCalls)
			b, _ := encoded(calls)
			if a != b {
				return tool.ErrReplay
			}
		} else {
			var valid bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tool_invocation i JOIN code_execution c ON c.execution_id=i.invocation_id WHERE i.invocation_id=? AND i.run_id=? AND i.provider_exchange_id=? AND i.tool_name='builtin__codemode__execute' AND i.status='running')`, parent, t.claim.RunID(), exchange).Scan(&valid); err != nil {
				return err
			}
			if !valid {
				return agent.ErrConflict
			}
		}
		for i, call := range calls {
			request, _ := encoded(call)
			if parent != "" {
				request, _ = encoded(struct {
					Call  llm.ToolCall
					Specs []tool.Spec
				}{call, specs})
			}
			hash := digest(request)
			var inv tool.Invocation
			var oldHash string
			var result sql.NullString
			err := tx.QueryRowContext(ctx, `SELECT invocation_id,status,request_hash,result_json FROM tool_invocation WHERE run_id=? AND provider_exchange_id=? AND provider_call_id=?`, t.claim.RunID(), exchange, call.ID).Scan(&inv.ID, &inv.Status, &oldHash, &result)
			inv.Call = call
			if err == nil {
				if oldHash != hash {
					return tool.ErrReplay
				}
				if result.Valid && json.Unmarshal([]byte(result.String), &inv.Result) != nil {
					return agent.ErrStorage
				}
			} else if err == sql.ErrNoRows {
				inv.ID = newID()
				inv.Status = "queued"
				now := time.Now().UnixMicro()
				policy := "never"
				p := tool.Spec{}.Policy()
				keys := []string{}
				for _, spec := range specs {
					if spec.Name == call.Name {
						p = spec.Policy()
						policy = p.Replay
						if d, ok := t.runs.registry.Resolve(spec.Name); ok && d.ConflictKeys != nil && tool.Validate(spec.InputSchema, call.Arguments) {
							keys = d.ConflictKeys(call.Arguments)
						}
					}
				}
				rawPolicy, _ := encoded(p)
				rawKeys, _ := encoded(keys)
				if len(keys) > 32 || len(rawKeys) > 8192 {
					return agent.ErrConflict
				}
				sort.Strings(keys)
				rawKeys, _ = encoded(keys)
				_, err = tx.ExecContext(ctx, `INSERT INTO tool_invocation(parent_invocation_id,invocation_id,run_id,provider_exchange_id,provider_call_id,tool_name,arguments_json,request_hash,replay_policy,idempotency_key,status,delivery_seq,created_at,updated_at,version,policy_json,conflict_keys) VALUES(?,?,?,?,?,?,?,?,?,?,'queued',?,?,?,1,?,?)`, nullable(parent), inv.ID, t.claim.RunID(), exchange, call.ID, call.Name, call.Arguments, hash, policy, newID(), i, now, now, rawPolicy, rawKeys)
				if err != nil {
					return err
				}
				if parent != "" {
					if _, err = tx.ExecContext(ctx, `INSERT INTO code_call(execution_id,call_id,invocation_id,request_hash) VALUES(?,?,?,?)`, parent, call.ID, inv.ID, hash); err != nil {
						return err
					}
				}
				if err = runEvent(ctx, tx, t.claim.RunID(), "tool_invocation.accepted", 1); err != nil {
					return err
				}
				allowed, err := t.runs.admitTreeCall(ctx, tx, t.claim.RunID())
				if err != nil {
					return err
				}
				rejection := tool.Result{}
				if !allowed {
					rejection = tool.Failure("RESOURCE_LIMIT")
				} else if call.Name == agent.RunToolName {
					rejection, err = t.runs.StartChildRun(ctx, tx, t.claim, inv)
					if err != nil {
						return err
					}
				}
				if rejection.Error != nil {
					value, _ := encoded(rejection)
					if _, err = tx.ExecContext(ctx, `UPDATE tool_invocation SET status='running',updated_at=?,version=version+1 WHERE invocation_id=?`, now, inv.ID); err != nil {
						return err
					}
					if _, err = tx.ExecContext(ctx, `UPDATE tool_invocation SET status='failed',result_json=?,result_ref=?,error_json=?,result_ready_at=?,updated_at=?,version=version+1 WHERE invocation_id=?`, value, digest(value), rejection.ModelText(), now, now, inv.ID); err != nil {
						return err
					}
					if err = runEvent(ctx, tx, t.claim.RunID(), "tool_invocation.failed", 1); err != nil {
						return err
					}
					inv.Status, inv.Result = "failed", rejection
				}

			} else {
				return err
			}
			out = append(out, inv)
		}
		t.runs.host.boundary("before_tool_accept_commit")
		return nil
	})
	if err == nil {
		t.runs.host.boundary("after_tool_accept_commit")
		for _, inv := range out {
			if inv.Call.Name == agent.RunToolName && inv.Status != "failed" {
				t.runs.host.boundary("after_child_accept_commit")
				break
			}
		}
	}
	return out, err
}
func (t *tools) Start(ctx context.Context, id string) error {
	return t.runs.write(ctx, func(tx *sql.Tx) error {
		if err := t.fence(ctx, tx); err != nil {
			return err
		}
		inv, p, err := t.load(ctx, tx, id)
		if err != nil {
			return err
		}
		if inv.Status == "running" {
			var owner sql.NullString
			if err = tx.QueryRowContext(ctx, `SELECT execution_token FROM tool_invocation WHERE invocation_id=?`, id).Scan(&owner); err != nil {
				return err
			}
			if owner.Valid && owner.String == t.claim.(*claim).token {
				return agent.ErrBusy
			}
		}

		if p.Risk == "high" {
			var valid int
			if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM approval a JOIN tool_invocation i USING(invocation_id) JOIN run r USING(run_id) WHERE a.invocation_id=? AND a.status='approved' AND a.expires_at>? AND a.scope=r.agent_id AND a.frozen_arguments_hash=i.request_hash`, id, t.runs.approvalTime().UnixMicro()).Scan(&valid); err != nil {
				return err
			}
			if valid != 1 {
				return tool.ErrApprovalDenied
			}
		}
		// The Code Mode wrapper coordinates nested ToolInvocations; it never
		// owns their execution slots. Counting it would deadlock a limit of one.
		// Its VM remains bounded by the Loop/Executor and JavaScript limits.
		if inv.Call.Name != codemode.ToolName {
			// Actual Tools share atomic quotas/conflicts across direct and nested
			// calls. Expired owners and wrappers retain no execution capacity.
			var global, byAgent, byRun, byProvider, byTool, conflicts int
			err = tx.QueryRowContext(ctx, `SELECT count(*),coalesce(sum(r.agent_id=(SELECT agent_id FROM run WHERE run_id=?)),0),coalesce(sum(i.run_id=?),0),coalesce(sum(json_extract(i.policy_json,'$.provider')=?),0),coalesce(sum(i.tool_name=?),0),coalesce(sum(EXISTS(SELECT 1 FROM json_each(i.conflict_keys) a JOIN json_each((SELECT conflict_keys FROM tool_invocation WHERE invocation_id=?)) b ON a.value=b.value)),0) FROM tool_invocation i JOIN run r USING(run_id) JOIN run_claim c USING(run_id) WHERE i.status='running' AND c.token IS NOT NULL AND c.expires_at>? AND i.invocation_id!=? AND i.tool_name!=?`, t.claim.RunID(), t.claim.RunID(), p.Provider, inv.Call.Name, id, time.Now().UnixMicro(), id, codemode.ToolName).Scan(&global, &byAgent, &byRun, &byProvider, &byTool, &conflicts)
			if err != nil {
				return err
			}
			limits := t.runs.toolLimits()
			if global >= limits.Global || byAgent >= limits.Agent || byRun >= limits.Run || byProvider >= limits.Provider || byTool >= limits.Tool || conflicts > 0 {
				return tool.ErrResource
			}
		}
		res, err := tx.ExecContext(ctx, `UPDATE tool_invocation SET status='running',wait_reason=NULL,next_wake_at=NULL,execution_token=?,attempt_count=attempt_count+1,version=version+1,updated_at=? WHERE invocation_id=? AND run_id=? AND desired_state='running' AND status IN ('queued','running','waiting')`, t.claim.(*claim).token, time.Now().UnixMicro(), id, t.claim.RunID())
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return agent.ErrConflict
		}
		return runEvent(ctx, tx, t.claim.RunID(), "tool_invocation.running", 1)
	})
}
func (t *tools) Finish(ctx context.Context, id string, result tool.Result) error {
	return t.finish(ctx, ctx, id, result, false, false, nil)
}
func (t *tools) Reject(ctx context.Context, id string, result tool.Result) error {
	if result.Error == nil {
		return agent.ErrConflict
	}
	return t.finish(ctx, ctx, id, result, false, true, nil)
}
func (t *tools) FinishResult(ctx context.Context, id string, result tool.Result) (tool.Result, error) {
	var out tool.Result
	err := t.finish(context.WithoutCancel(ctx), ctx, id, result, false, false, &out)
	return out, err
}
func (t *tools) finish(ctx, outputCtx context.Context, id string, result tool.Result, reconcile, reject bool, out *tool.Result) error {
	committed := false
	err := t.runs.write(ctx, func(tx *sql.Tx) error {
		if err := t.fence(ctx, tx); err != nil {
			return err
		}
		// A wrapper result can commit only under the same execution owner that
		// produced its frozen terminal receipt. Calls rejected before VM admission
		// have no CodeExecutionState and follow ordinary Tool rejection semantics.
		owner, _ := result.HostFence.(codeClaimFence)
		var wrongOwner bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM code_execution_state WHERE execution_id=? AND (claim_token IS NULL OR claim_token!=? OR execution_epoch!=? OR owner_run_token!=? OR claim_expires_at<=?))`, id, owner.token, owner.epoch, t.claim.(*claim).token, time.Now().UnixMicro()).Scan(&wrongOwner); err != nil {
			return err
		}
		if wrongOwner {
			return agent.ErrClaimLost
		}
		if reject {
			if _, err := tx.ExecContext(ctx, `UPDATE tool_invocation SET status='running',wait_reason=NULL,updated_at=?,version=version+1 WHERE invocation_id=? AND run_id=? AND status IN ('queued','waiting')`, time.Now().UnixMicro(), id, t.claim.RunID()); err != nil {
				return err
			}
		}
		result = tool.Normalize(result)
		var err error
		if reconcile {
			result, err = t.runs.storeRecoveryOutput(ctx, outputCtx, tx, t.claim.RunID(), id, result)
		} else {
			result, err = t.runs.storeOutput(ctx, outputCtx, tx, t.claim.RunID(), id, result)
		}
		if err != nil {
			return err
		}
		if err = t.fence(ctx, tx); err != nil {
			return err
		}
		if out != nil {
			*out = result
		}
		if reconcile && result.Error != nil && result.Error.Code == "UNKNOWN_SIDE_EFFECT" {
			return nil // Caller persists uncertainty for a still-running invocation.
		}
		value, err := encoded(result)
		if err != nil {
			return err
		}
		status := "succeeded"
		var errorJSON any
		if result.Error != nil {
			status = "failed"
			if result.Error.Code == "UNKNOWN_SIDE_EFFECT" {
				status = "unknown"
			}
			errorJSON, _ = encoded(result.Error)
		}
		now := time.Now().UnixMicro()
		var proof any
		if reconcile {
			proof = value
		}
		resultRef := digest(value)
		if len(result.ArtifactRefs) > 0 {
			resultRef = result.ArtifactRefs[0]
		}
		res, err := tx.ExecContext(ctx, `UPDATE tool_invocation SET status=?,wait_reason=NULL,result_json=?,result_ref=?,error_json=?,result_ready_at=?,updated_at=?,version=version+1,reconciliation_json=? WHERE invocation_id=? AND run_id=? AND (status='running' OR (? AND status='unknown'))`, status, value, resultRef, errorJSON, now, now, proof, id, t.claim.RunID(), reconcile)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			return agent.ErrConflict
		}
		committed = true
		t.runs.host.boundary("before_tool_commit")
		return runEvent(ctx, tx, t.claim.RunID(), "tool_invocation."+status, 1)
	})
	if err == nil && committed {
		t.runs.host.boundary("after_tool_commit")
	}
	return err
}

// load resolves identity only under the bound Run. No model IDs are accepted.
func (t *tools) load(ctx context.Context, tx *sql.Tx, id string) (tool.Invocation, tool.Policy, error) {
	var inv tool.Invocation
	var policy, result string
	err := tx.QueryRowContext(ctx, `SELECT invocation_id,tool_name,arguments_json,provider_call_id,status,coalesce(wait_reason,''),idempotency_key,attempt_count,coalesce(result_json,'{}'),policy_json FROM tool_invocation WHERE invocation_id=? AND run_id=?`, id, t.claim.RunID()).Scan(&inv.ID, &inv.Call.Name, &inv.Call.Arguments, &inv.Call.ID, &inv.Status, &inv.WaitReason, &inv.Key, &inv.Attempts, &result, &policy)
	var p tool.Policy
	if err != nil {
		return inv, p, err
	}
	if json.Unmarshal([]byte(result), &inv.Result) != nil || json.Unmarshal([]byte(policy), &p) != nil {
		return inv, p, agent.ErrStorage
	}
	if p.Version == 0 { // Pre-M3 accepted safe/internal calls have fixed defaults.
		if d, ok := t.runs.registry.Resolve(inv.Call.Name); ok {
			p = d.Spec.Policy()
		}
	}
	return inv, p, nil
}
func (t *tools) Prepare(ctx context.Context, id string) (tool.Invocation, error) {
	var inv tool.Invocation
	err := t.runs.write(ctx, func(tx *sql.Tx) error {
		if err := t.fence(ctx, tx); err != nil {
			return err
		}
		var p tool.Policy
		var err error
		inv, p, err = t.load(ctx, tx, id)
		if err != nil {
			return err
		}
		if inv.Status == "running" {
			var owner sql.NullString
			if err = tx.QueryRowContext(ctx, `SELECT execution_token FROM tool_invocation WHERE invocation_id=?`, id).Scan(&owner); err != nil {
				return err
			}
			if owner.Valid && owner.String == t.claim.(*claim).token {
				return agent.ErrBusy
			}
		}

		if inv.Status == "succeeded" || inv.Status == "failed" || inv.Status == "cancelled" || inv.Status == "unknown" {
			return nil
		}
		// Resolve only within the immutable Run capability. A hidden Tool
		// must not reveal its schema through a validation error.
		var visible bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM run r JOIN capability_snapshot c ON c.snapshot_id=r.capability_snapshot_id,json_each(c.tools_json) s WHERE r.run_id=? AND json_extract(s.value,'$.name')=?)`, t.claim.RunID(), inv.Call.Name).Scan(&visible); err != nil {
			return err
		}
		var d tool.Definition
		known := false
		if visible {
			d, known = t.runs.registry.Resolve(inv.Call.Name)
		}
		rejection := ""
		if !known {
			rejection = "NOT_FOUND"
		} else if !tool.Validate(d.Spec.InputSchema, inv.Call.Arguments) {
			rejection = "INVALID_ARGUMENT"
		}
		if rejection != "" {
			// A rejection still uses the single accepted invocation lifecycle.
			inv.Result = tool.Failure(rejection)
			return nil
		}
		if inv.Status == "waiting" && inv.WaitReason == "retry_backoff" {
			var ready bool
			if err = tx.QueryRowContext(ctx, `SELECT coalesce(next_wake_at,0)<=? FROM tool_invocation WHERE invocation_id=?`, time.Now().UnixMicro(), id).Scan(&ready); err != nil {
				return err
			}
			if !ready {
				return nil
			}
			inv.Status = "queued" // Persisted retry intent proves no effect took place.
			return nil
		}
		if inv.Status == "waiting" && inv.WaitReason == "external_event" && inv.Call.Name == "builtin__codemode__execute" {
			inv.Status = "queued"
			return nil
		}
		if inv.Status == "waiting" && inv.WaitReason == "child_run" {
			var terminal bool
			if err = tx.QueryRowContext(ctx, `SELECT c.status IN ('completed','failed','cancelled') FROM agent_invocation a JOIN run c ON c.run_id=a.child_run_id WHERE a.invocation_id=?`, id).Scan(&terminal); err != nil {
				return err
			}
			if terminal {
				inv.Status = "queued"
			}
			return nil
		}
		if inv.Status == "waiting" && inv.WaitReason == "resource" {
			inv.Status = "queued"
		}
		if p.Risk != "high" || inv.Attempts > 0 {
			return nil
		}
		now := t.runs.approvalTime().UnixMicro()
		inserted, err := tx.ExecContext(ctx, `INSERT INTO approval(approval_id,invocation_id,risk_type,scope,frozen_arguments_hash,status,expires_at,created_at,version) SELECT ?,invocation_id,?,r.agent_id,request_hash,'pending',?,?,1 FROM tool_invocation i JOIN run r USING(run_id) WHERE invocation_id=? ON CONFLICT(invocation_id) DO NOTHING`, newID(), p.Risk, now+t.runs.approvalDuration().Microseconds(), now, id)
		if err != nil {
			return err
		}
		if n, _ := inserted.RowsAffected(); n > 0 {
			var approvalID, args, hash, name string
			if err = tx.QueryRowContext(ctx, `SELECT a.approval_id,i.arguments_json,a.frozen_arguments_hash,i.tool_name FROM approval a JOIN tool_invocation i USING(invocation_id) WHERE i.invocation_id=?`, id).Scan(&approvalID, &args, &hash, &name); err != nil {
				return err
			}
			text := "Approval required for " + name + ": " + args + "\nUse /approve " + approvalID + " " + hash + " or /deny " + approvalID + " " + hash
			if err = t.runs.notification(ctx, tx, t.claim.RunID(), "approval:"+approvalID, "approval", "security", text); err != nil {
				return err
			}
			if err = runEvent(ctx, tx, t.claim.RunID(), "approval.requested", 1); err != nil {
				return err
			}
		}
		var status string
		if err = tx.QueryRowContext(ctx, `SELECT status FROM approval WHERE invocation_id=?`, id).Scan(&status); err != nil {
			return err
		}
		if status == "pending" || status == "approved" {
			expired, err := tx.ExecContext(ctx, `UPDATE approval SET status='expired',decided_at=?,version=version+1 WHERE invocation_id=? AND status IN ('pending','approved') AND expires_at<=?`, now, id, now)
			if err != nil {
				return err
			}
			if n, _ := expired.RowsAffected(); n > 0 {
				if err = runEvent(ctx, tx, t.claim.RunID(), "approval.expired", 1); err != nil {
					return err
				}
			}
			if err = tx.QueryRowContext(ctx, `SELECT status FROM approval WHERE invocation_id=?`, id).Scan(&status); err != nil {
				return err
			}
		}
		if status == "approved" {
			inv.Status = "queued"
			return nil
		}
		if status != "pending" {
			inv.Result = tool.Failure("APPROVAL_DENIED")
			return nil
		}
		if inv.Status != "waiting" {
			if _, err = tx.ExecContext(ctx, `UPDATE tool_invocation SET status='waiting',wait_reason='approval',updated_at=?,version=version+1 WHERE invocation_id=?`, now, id); err != nil {
				return err
			}
			if err = runEvent(ctx, tx, t.claim.RunID(), "tool_invocation.waiting", 1); err != nil {
				return err
			}
		}
		inv.Status = "waiting"
		inv.WaitReason = "approval"
		return nil
	})
	// Persist rejection before returning it; no provider or resource acquisition.
	if err == nil && inv.Result.Error != nil && inv.Status != "succeeded" && inv.Status != "failed" && inv.Status != "cancelled" && inv.Status != "unknown" {
		err = t.Reject(ctx, id, inv.Result)
		if err == nil {
			inv.Status = "failed"
		}
	}
	return inv, err
}
func (t *tools) Defer(ctx context.Context, id, reason string, wake time.Time) error {
	if reason != "resource" && reason != "retry_backoff" {
		return agent.ErrConflict
	}
	return t.runs.write(ctx, func(tx *sql.Tx) error {
		if err := t.fence(ctx, tx); err != nil {
			return err
		}
		var at any
		if !wake.IsZero() {
			at = wake.UnixMicro()
		}
		res, err := tx.ExecContext(ctx, `UPDATE tool_invocation SET status='waiting',wait_reason=?,next_wake_at=?,updated_at=?,version=version+1 WHERE invocation_id=? AND run_id=? AND status IN ('queued','running','waiting')`, reason, at, time.Now().UnixMicro(), id, t.claim.RunID())
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return agent.ErrConflict
		}
		return runEvent(ctx, tx, t.claim.RunID(), "tool_invocation.waiting", 1)
	})
}
func (t *tools) Unknown(ctx context.Context, id string) error {
	return t.runs.write(ctx, func(tx *sql.Tx) error {
		if err := t.fence(ctx, tx); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE tool_invocation SET status='unknown',wait_reason=NULL,error_json=?,updated_at=?,version=version+1 WHERE invocation_id=? AND run_id=? AND status='running'`, tool.Failure("UNKNOWN_SIDE_EFFECT").ModelText(), time.Now().UnixMicro(), id, t.claim.RunID())
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return agent.ErrConflict
		}
		return runEvent(ctx, tx, t.claim.RunID(), "tool_invocation.unknown", 1)
	})
}
func (t *tools) Reconcile(ctx context.Context, id string, result tool.Result) error {
	_, err := t.ReconcileResult(ctx, id, result)
	return err
}
func (t *tools) ReconcileResult(ctx context.Context, id string, result tool.Result) (tool.Result, error) {
	var out tool.Result
	err := t.finish(context.WithoutCancel(ctx), ctx, id, result, true, false, &out)
	return out, err
}

var _ tool.PolicyStore = (*tools)(nil)

func (t *tools) WaitReason(ctx context.Context) (string, error) {
	// Any resource/retry wait needs a scheduler wake even if another call is
	// awaiting approval. Approval-only batches release the Run indefinitely.
	var pending int
	if err := t.runs.host.db.QueryRowContext(ctx, `SELECT count(*) FROM tool_invocation WHERE run_id=? AND status='waiting' AND wait_reason!='approval'`, t.claim.RunID()).Scan(&pending); err != nil {
		return "", agent.ErrStorage
	}
	if pending > 0 {
		return "resource", nil
	}
	return "approval", nil
}
