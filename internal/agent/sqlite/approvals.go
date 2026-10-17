package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/z-chenhao/erii/internal/agent"
	"github.com/z-chenhao/erii/internal/tool"
)

type ToolLimits struct{ Global, Agent, Run, Provider, Tool int }

func (r *Runs) ConfigureToolLimits(l ToolLimits)       { r.limits = l }
func (r *Runs) ConfigureApprovalTTL(ttl time.Duration) { r.approvalTTL = ttl }
func (r *Runs) approvalTime() time.Time {
	if r.approvalClock != nil {
		return r.approvalClock()
	}
	return time.Now()
}
func (r *Runs) approvalDuration() time.Duration {
	if r.approvalTTL > 0 {
		return r.approvalTTL
	}
	return time.Hour
}
func (r *Runs) toolLimits() ToolLimits {
	l := r.limits
	if l.Global < 1 {
		l.Global = 32
	}
	if l.Agent < 1 {
		l.Agent = 24
	}
	if l.Run < 1 {
		l.Run = 4
	}
	if l.Provider < 1 {
		l.Provider = 16
	}
	if l.Tool < 1 {
		l.Tool = 8
	}
	return l
}

// ApprovalView belongs to the local user control plane, never model context.
// The local CLI is authenticated by ownership of the private data directory.
type ApprovalView struct {
	ID        string `json:"approval_id"`
	Tool      string `json:"tool"`
	Arguments string `json:"arguments"`
	Hash      string `json:"frozen_arguments_hash"`
	Status    string `json:"status"`
	ExpiresAt int64  `json:"expires_at"`
}

func (r *Runs) Approvals(ctx context.Context) ([]ApprovalView, error) {
	if err := r.recoverApprovals(ctx); err != nil {
		return nil, err
	}
	rows, err := r.host.db.QueryContext(ctx, `SELECT approval_id,tool_name,arguments_json,frozen_arguments_hash,status,expires_at FROM (
 SELECT a.approval_id,i.tool_name,i.arguments_json,a.frozen_arguments_hash,a.status,a.expires_at,a.created_at FROM approval a JOIN tool_invocation i USING(invocation_id) WHERE a.scope=(SELECT agent_id FROM agent_config WHERE singleton=1)
 UNION ALL SELECT approval_id,'account_spend',arguments_json,frozen_hash,status,0,created_at FROM spend_approval WHERE scope=(SELECT agent_id FROM agent_config WHERE singleton=1)
 ) ORDER BY created_at,approval_id`)
	if err != nil {
		return nil, agent.ErrStorage
	}
	defer rows.Close()
	out := []ApprovalView{}
	for rows.Next() {
		var v ApprovalView
		if rows.Scan(&v.ID, &v.Tool, &v.Arguments, &v.Hash, &v.Status, &v.ExpiresAt) != nil {
			return nil, agent.ErrStorage
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Decide requires the displayed frozen hash, in addition to the approval ID.
// A duplicate matching decision is idempotent; conflicting callbacks fail.
func (r *Runs) Decide(ctx context.Context, principal, id, hash, decision string) error {
	if decision != "approved" && decision != "denied" {
		return agent.ErrConflict
	}
	if err := r.recoverApprovals(ctx); err != nil {
		return err
	}
	return r.write(ctx, func(tx *sql.Tx) error {
		if found, err := r.decideSpend(ctx, tx, principal, id, hash, decision); found || err != nil {
			return err
		}
		var status, runID string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM approval WHERE approval_id=? AND scope=? AND frozen_arguments_hash=?`, id, principal, hash).Scan(&status); err != nil {
			return agent.ErrConflict
		}
		if status == decision {
			return nil
		}
		err := tx.QueryRowContext(ctx, `SELECT a.status,i.run_id FROM approval a JOIN tool_invocation i USING(invocation_id) JOIN run r USING(run_id) WHERE a.approval_id=? AND a.scope=? AND r.agent_id=? AND a.frozen_arguments_hash=? AND i.request_hash=? AND i.status='waiting' AND i.wait_reason='approval' AND r.desired_state='running' AND r.status IN ('running','waiting') AND a.expires_at>?`, id, principal, principal, hash, hash, r.approvalTime().UnixMicro()).Scan(&status, &runID)
		if err != nil {
			return agent.ErrConflict
		}
		if status == decision {
			return nil
		}
		if status != "pending" {
			return agent.ErrConflict
		}
		if _, err = tx.ExecContext(ctx, `UPDATE approval SET status=?,decision_source='local_user',decided_at=?,version=version+1 WHERE approval_id=?`, decision, r.approvalTime().UnixMicro(), id); err != nil {
			return err
		}
		if err = wakeApproval(ctx, tx, runID); err != nil {
			return err
		}
		return runEvent(ctx, tx, runID, "approval.decided", 1)
	})
}
func wakeApproval(ctx context.Context, tx *sql.Tx, runID string) error {
	// Only released approval waiters need an explicit wake. Running owners see
	// the decision in Prepare; other wait reasons retain their own wake policy.
	_, err := tx.ExecContext(ctx, `UPDATE run SET status='running',wait_reason=NULL,updated_at=?,version=version+1 WHERE run_id=? AND status='waiting' AND wait_reason='approval' AND desired_state='running'`, time.Now().UnixMicro(), runID)
	return err
}
func (r *Runs) recoverApprovals(ctx context.Context) error {
	return r.write(ctx, func(tx *sql.Tx) error {
		now := r.approvalTime().UnixMicro()
		rows, err := tx.QueryContext(ctx, `UPDATE approval SET status='expired',decided_at=?,version=version+1 WHERE status IN ('pending','approved') AND expires_at<=? AND invocation_id IN (SELECT invocation_id FROM tool_invocation WHERE status IN ('queued','waiting') AND attempt_count=0) RETURNING invocation_id`, now, now)
		if err != nil {
			return err
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, id := range ids {
			var run string
			if err = tx.QueryRowContext(ctx, `SELECT run_id FROM tool_invocation WHERE invocation_id=?`, id).Scan(&run); err != nil {
				return err
			}
			if err = wakeApproval(ctx, tx, run); err != nil {
				return err
			}
			if err = runEvent(ctx, tx, run, "approval.expired", 1); err != nil {
				return err
			}
		}
		return nil
	})
}

// Steering invalidates every unexecuted approval in that Run, including a
// decision received before dispatch. New arguments require a new invocation.
func invalidateApprovals(ctx context.Context, tx *sql.Tx, runID string) error {
	res, err := tx.ExecContext(ctx, `UPDATE approval SET status='invalidated',decided_at=?,version=version+1 WHERE status IN ('pending','approved') AND invocation_id IN (SELECT invocation_id FROM tool_invocation WHERE run_id=? AND attempt_count=0 AND status IN ('queued','waiting'))`, time.Now().UnixMicro(), runID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil
	}
	if err = wakeApproval(ctx, tx, runID); err != nil {
		return err
	}
	return runEvent(ctx, tx, runID, "approval.invalidated", 1)
}

// ReconcileUnknown queries only; it never calls Execute/Invoke. It can settle
// an unknown after its Run ended, without reviving or modifying that Run.
func (r *Runs) ReconcileUnknown(ctx context.Context) error {
	rows, err := r.host.db.QueryContext(ctx, `SELECT i.invocation_id,i.run_id,i.tool_name,i.arguments_json,i.idempotency_key,c.tools_json FROM tool_invocation i JOIN run r USING(run_id) JOIN capability_snapshot c ON c.snapshot_id=r.capability_snapshot_id WHERE i.status='unknown' ORDER BY i.created_at`)
	if err != nil {
		return agent.ErrStorage
	}
	type item struct{ id, run, name, args, key, specs string }
	items := []item{}
	for rows.Next() {
		var v item
		if err = rows.Scan(&v.id, &v.run, &v.name, &v.args, &v.key, &v.specs); err != nil {
			rows.Close()
			return err
		}
		items = append(items, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range items {
		var specs []tool.Spec
		if json.Unmarshal([]byte(v.specs), &specs) != nil {
			return agent.ErrStorage
		}
		registry, err := r.registry.Select(specs)
		if err != nil {
			return err
		}
		d, ok := registry.Resolve(v.name)
		if !ok || d.Query == nil {
			continue
		}
		_, err = tool.RecoverResult(ctx, d, tool.Request{Arguments: v.args, IdempotencyKey: v.key}, func(outputCtx context.Context, result tool.Result) (tool.Result, error) {
			writeCtx := context.WithoutCancel(outputCtx)
			committed := false
			err := r.write(writeCtx, func(tx *sql.Tx) error {
				// The query ran outside the lock. Another reconciler may have won;
				// never consume or publish another receipt after terminal commit.
				var status string
				if err := tx.QueryRowContext(writeCtx, `SELECT status FROM tool_invocation WHERE invocation_id=?`, v.id).Scan(&status); err != nil {
					return err
				}
				if status != "unknown" {
					result = tool.Failure("UNKNOWN_SIDE_EFFECT")
					return nil
				}
				var err error
				result, err = r.storeRecoveryOutput(writeCtx, outputCtx, tx, v.run, v.id, result)
				if err != nil || (result.Error != nil && result.Error.Code == "UNKNOWN_SIDE_EFFECT") {
					return err
				}
				value, _ := encoded(result)
				status = "succeeded"
				var errorJSON any
				if result.Error != nil {
					status = "failed"
					errorJSON, _ = encoded(result.Error)
				}
				resultRef := digest(value)
				if len(result.ArtifactRefs) > 0 {
					resultRef = result.ArtifactRefs[0]
				}
				res, err := tx.ExecContext(writeCtx, `UPDATE tool_invocation SET status=?,result_json=?,result_ref=?,error_json=?,result_ready_at=?,updated_at=?,reconciliation_json=?,version=version+1 WHERE invocation_id=? AND status='unknown'`, status, value, resultRef, errorJSON, time.Now().UnixMicro(), time.Now().UnixMicro(), value, v.id)
				if err != nil {
					return err
				}
				if n, _ := res.RowsAffected(); n == 0 {
					return nil
				}
				r.host.boundary("before_tool_commit")
				committed = true
				return runEvent(writeCtx, tx, v.run, "tool_invocation.reconciled", 1)
			})
			if err == nil && committed {
				r.host.boundary("after_tool_commit")
			}
			return result, err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// ToolOutcomes exposes query-only reconciliation observations to the local user.
// It does not mutate or redeliver historical model/Conversation messages.
type ToolOutcome struct {
	Tool      string      `json:"tool"`
	Arguments string      `json:"arguments"`
	Status    string      `json:"status"`
	Result    tool.Result `json:"result"`
}

func (r *Runs) ToolOutcomes(ctx context.Context) ([]ToolOutcome, error) {
	rows, err := r.host.db.QueryContext(ctx, `SELECT i.tool_name,i.arguments_json,i.status,coalesce(i.result_json,'{}') FROM tool_invocation i JOIN run r USING(run_id) WHERE r.agent_id=(SELECT agent_id FROM agent_config WHERE singleton=1) AND (i.status='unknown' OR i.reconciliation_json IS NOT NULL) ORDER BY i.created_at,i.invocation_id`)
	if err != nil {
		return nil, agent.ErrStorage
	}
	defer rows.Close()
	out := []ToolOutcome{}
	for rows.Next() {
		var v ToolOutcome
		var result string
		if err = rows.Scan(&v.Tool, &v.Arguments, &v.Status, &result); err != nil {
			return nil, agent.ErrStorage
		}
		if json.Unmarshal([]byte(result), &v.Result) != nil {
			return nil, agent.ErrStorage
		}
		if v.Status == "unknown" {
			v.Result = tool.Failure("UNKNOWN_SIDE_EFFECT")
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
