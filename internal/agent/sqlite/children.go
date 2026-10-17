package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/z-chenhao/erii/internal/agent"
	"github.com/z-chenhao/erii/internal/skill"
	"github.com/z-chenhao/erii/internal/tool"
)

func (r *Runs) ConfigureChildLimits(l agent.ChildLimits) { r.childLimits = l }
func (r *Runs) createTree(ctx context.Context, tx *sql.Tx, root string) error {
	l := r.childLimits
	if !l.Valid() {
		l = agent.DefaultChildLimits()
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO run_tree(root_run_id,max_depth,max_descendants,max_tool_calls,max_turns,version) VALUES(?,?,?,?,?,1)`, root, l.Depth, l.Descendants, l.ToolCalls, r.schedulerLimits().Turns)
	return err
}

// admitTreeCall executes inside first ToolInvocation admission, never on replay.
func (r *Runs) admitTreeCall(ctx context.Context, tx *sql.Tx, runID string) (bool, error) {
	var kind string
	if err := tx.QueryRowContext(ctx, `SELECT kind FROM run WHERE run_id=?`, runID).Scan(&kind); err != nil {
		return false, err
	}
	if kind != "worker" {
		return true, nil
	}
	res, err := tx.ExecContext(ctx, `UPDATE run_tree SET tool_calls=tool_calls+1,version=version+1 WHERE root_run_id=(SELECT root_run_id FROM run WHERE run_id=?) AND tool_calls<max_tool_calls`, runID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// StartChildRun is AgentService's admission operation. It participates in the
// existing Tool Store transaction: invocation + Child + counter + outbox commit
// together. The only caller holds a fenced Parent Worker claim and a freshly
// inserted, provider-accepted invocation. No external effect is dispatched here.
func (r *Runs) StartChildRun(ctx context.Context, tx *sql.Tx, c agent.RunClaim, inv tool.Invocation) (tool.Result, error) {
	if err := fence(ctx, tx, c); err != nil {
		return tool.Result{}, err
	}
	// Resolve trusted identity from the accepted record, never caller fields.
	if err := tx.QueryRowContext(ctx, `SELECT tool_name,arguments_json FROM tool_invocation WHERE invocation_id=? AND run_id=? AND status='queued'`, inv.ID, c.RunID()).Scan(&inv.Call.Name, &inv.Call.Arguments); err != nil {
		return tool.Result{}, err
	}
	if inv.Call.Name != agent.RunToolName {
		return tool.Failure("PERMISSION_DENIED"), nil
	}
	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM agent_invocation WHERE invocation_id=?`, inv.ID).Scan(&existing); err != nil {
		return tool.Result{}, err
	}
	if existing == 1 {
		return tool.Result{}, nil
	}
	var kind, taskID, rootID, agentID, conv, modelID, capID, raw, scope string
	var depth, epoch, config, format int64
	var spawn bool
	err := tx.QueryRowContext(ctx, `SELECT r.kind,coalesce(r.task_id,''),coalesce(r.root_run_id,''),r.agent_id,r.conversation_id,r.model_snapshot_id,r.depth,coalesce(r.task_epoch,0),s.tools_json,s.can_spawn_agents,s.data_scope,s.agent_config_version,s.format_version FROM run r JOIN capability_snapshot s ON s.snapshot_id=r.capability_snapshot_id WHERE r.run_id=?`, c.RunID()).Scan(&kind, &taskID, &rootID, &agentID, &conv, &modelID, &depth, &epoch, &raw, &spawn, &scope, &config, &format)
	if err != nil {
		return tool.Result{}, err
	}
	var specs []tool.Spec
	if json.Unmarshal([]byte(raw), &specs) != nil {
		return tool.Result{}, agent.ErrStorage
	}
	visible := false
	for _, s := range specs {
		visible = visible || (s.Name == agent.RunToolName && s.Permission == "spawn_agents")
	}
	if !visible {
		return tool.Failure("NOT_FOUND"), nil
	}
	if kind != "worker" || !spawn {
		return tool.Failure("PERMISSION_DENIED"), nil
	}
	req, err := agent.ParseAgentRun(inv.Call.Arguments)
	if err != nil {
		return tool.Failure("INVALID_ARGUMENT"), nil
	}
	parentScope, err := runScope(ctx, tx, c.RunID())
	if err != nil {
		return tool.Result{}, err
	}
	for _, ref := range req.ArtifactRefs {
		if _, e := r.host.Artifacts().DescribeTx(ctx, tx, parentScope, ref, true); e != nil {
			return artifactFailure(e), nil
		}
	}
	res, err := tx.ExecContext(ctx, `UPDATE run_tree SET descendants=descendants+1,version=version+1 WHERE root_run_id=? AND descendants<max_descendants AND ?>=0 AND ?<=max_depth`, rootID, depth, depth+1)
	if err != nil {
		return tool.Result{}, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return tool.Failure("RESOURCE_LIMIT"), nil
	}
	specs = agent.WorkerTools(specs)
	raw, _ = encoded(specs)
	id := newID()
	capID = newID()
	now := time.Now().UnixMicro()
	if _, err = tx.ExecContext(ctx, `INSERT INTO capability_snapshot(snapshot_id,agent_config_version,role,tool_specs_hash,skill_set_hash,data_scope,can_spawn_agents,tools_json,created_at,format_version) VALUES(?,?,'worker',?,?,?,?,?,?,?)`, capID, config, digest(raw), skill.SetHash(specs), scope, spawn, raw, now, format); err != nil {
		return tool.Result{}, err
	}
	cp, _ := encoded(agent.Checkpoint{ContextReady: true, Phase: "model", Messages: agent.ChildContext(req)})
	if _, err = tx.ExecContext(ctx, `INSERT INTO run(run_id,agent_id,conversation_id,task_id,parent_run_id,root_run_id,kind,worker_position,status,task_epoch,depth,capability_snapshot_id,model_snapshot_id,checkpoint_json,history_boundary,created_at,updated_at,version) VALUES(?,?,?,?,?,?,'worker','child','queued',?,?,?,?,?,0,?,?,1)`, id, agentID, conv, taskID, c.RunID(), rootID, epoch, depth+1, capID, modelID, cp, now, now); err != nil {
		return tool.Result{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO run_claim VALUES(?,0,NULL,0,1)`, id); err != nil {
		return tool.Result{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agent_invocation VALUES(?,?,?)`, inv.ID, id, digest(inv.Call.Arguments)); err != nil {
		return tool.Result{}, err
	}
	for _, ref := range req.ArtifactRefs {
		if err = r.host.Artifacts().PinTx(ctx, tx, parentScope, ref, "run", id); err != nil {
			return tool.Result{}, err
		}
	}
	if err = runEvent(ctx, tx, id, "run_ready", 1); err != nil {
		return tool.Result{}, err
	}
	r.host.boundary("before_child_accept_commit")
	return tool.Result{}, nil
}

// AgentToolExecutor joins the Child already created at admission. Waiting uses
// ordinary ToolInvocation suspension, releasing the Parent lease and resources.
func (t *tools) AgentToolExecutor(ctx context.Context, id string) (tool.Result, error) {
	result := tool.Result{}
	waiting := false
	err := t.runs.write(ctx, func(tx *sql.Tx) error {
		waiting = false
		if err := t.fence(ctx, tx); err != nil {
			return err
		}
		var child, status, name string
		if err := tx.QueryRowContext(ctx, `SELECT a.child_run_id,c.status,i.tool_name FROM agent_invocation a JOIN tool_invocation i USING(invocation_id) JOIN run c ON c.run_id=a.child_run_id WHERE i.invocation_id=? AND i.run_id=? AND i.status='running' AND c.parent_run_id=i.run_id`, id, t.claim.RunID()).Scan(&child, &status, &name); err != nil {
			return err
		}
		if name != agent.RunToolName {
			return agent.ErrConflict
		}
		if status != "completed" && status != "failed" && status != "cancelled" {
			if _, err := tx.ExecContext(ctx, `UPDATE tool_invocation SET status='waiting',wait_reason='child_run',updated_at=?,version=version+1 WHERE invocation_id=? AND run_id=? AND status='running'`, time.Now().UnixMicro(), id, t.claim.RunID()); err != nil {
				return err
			}
			waiting = true
			return runEvent(ctx, tx, t.claim.RunID(), "tool_invocation.waiting", 1)
		}
		// An uncertain Child effect remains uncertain at the Parent boundary.
		var unknown int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM tool_invocation WHERE run_id=? AND status='unknown'`, child).Scan(&unknown); err != nil {
			return err
		}
		if unknown > 0 {
			result = tool.Failure("UNKNOWN_SIDE_EFFECT")
			return nil
		}
		if status != "completed" {
			code := "PROVIDER_UNAVAILABLE"
			if status == "cancelled" {
				code = "CANCELLED"
			} else {
				var cause string
				if err := tx.QueryRowContext(ctx, `SELECT coalesce(error_code,'') FROM run WHERE run_id=?`, child).Scan(&cause); err != nil {
					return err
				}
				if cause == "RESOURCE_LIMIT" {
					code = cause
				}
			}
			result = tool.Failure(code)
			return nil
		}
		var text string
		if err := tx.QueryRowContext(ctx, `SELECT content FROM worker_result WHERE run_id=? ORDER BY created_at DESC,exchange_id DESC LIMIT 1`, child).Scan(&text); err != nil {
			return err
		}
		refs, err := published(ctx, tx, child)
		if err != nil {
			return err
		}
		if refs == nil {
			refs = []string{}
		}
		scope, err := runScope(ctx, tx, child)
		if err != nil {
			return err
		}
		for _, ref := range refs {
			if err = t.runs.host.Artifacts().PinTx(ctx, tx, scope, ref, "run", t.claim.RunID()); err != nil {
				return err
			}
		}
		raw, err := encoded(agent.AgentResult{Text: text, ArtifactRefs: refs})
		result = tool.Result{Text: raw}
		return err
	})
	if err != nil {
		return tool.Result{}, err
	}
	if waiting {
		return tool.Result{}, tool.ErrSuspended
	}
	return result, nil
}

func (r *Runs) childFinished(ctx context.Context, tx *sql.Tx, id string) error {
	var parent sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT parent_run_id FROM run WHERE run_id=?`, id).Scan(&parent); err != nil {
		return err
	}
	if !parent.Valid {
		return nil
	}
	// Wake is an outbox hint; authoritative child status is scanned after restart
	// and before every Parent claim, including completion-before-wait races.
	return runEvent(ctx, tx, parent.String, "run_ready", 1)
}
