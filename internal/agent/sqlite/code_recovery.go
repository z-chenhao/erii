package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/z-chenhao/erii/internal/agent"
	"github.com/z-chenhao/erii/internal/tool"
	"github.com/z-chenhao/erii/internal/tool/codemode"
)

// The execution lease is subordinate to the Run lease. Neither can authorize
// callbacks from a prior VM owner, even while the same Run is still alive.
func (b *codeBridge) fence(ctx context.Context, tx *sql.Tx) error {
	if err := fence(ctx, tx, b.t.claim); err != nil {
		return err
	}
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM code_execution_state WHERE execution_id=? AND claim_token=? AND execution_epoch=? AND claim_expires_at>? AND desired_state='running' AND state='running')`, b.req.ExecutionID, b.req.ClaimToken, b.req.ExecutionEpoch, time.Now().UnixMicro()).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return agent.ErrClaimLost
	}
	return nil
}

type codeClaimFence struct {
	token string
	epoch int64
}

func (b *codeBridge) claim(ctx context.Context, inv tool.Invocation) error {
	drift := false
	err := b.t.runs.write(ctx, func(tx *sql.Tx) error {
		if err := fence(ctx, tx, b.t.claim); err != nil {
			return err
		}
		req := &b.req
		var raw, capID, modelID string
		if err := tx.QueryRowContext(ctx, `SELECT r.kind,c.tools_json,i.provider_exchange_id,i.provider_call_id,r.capability_snapshot_id,r.model_snapshot_id FROM run r JOIN capability_snapshot c ON c.snapshot_id=r.capability_snapshot_id JOIN tool_invocation i USING(run_id) WHERE i.invocation_id=? AND i.status='running'`, inv.ID).Scan(&req.Kind, &raw, &b.exchange, &req.ProviderCallID, &capID, &modelID); err != nil {
			return err
		}
		if json.Unmarshal([]byte(raw), &req.Tools) != nil {
			return agent.ErrStorage
		}
		// Include identities and complete frozen capability JSON, not mutable profile metadata.
		snapshots := digest(capID + ":" + modelID + ":" + raw)
		manifest, _ := encoded(*req) // Claim fields are empty until identity is frozen.
		now := time.Now().UnixMicro()
		inserted, err := tx.ExecContext(ctx, `INSERT INTO code_execution VALUES(?,?,?,?,?) ON CONFLICT DO NOTHING`, inv.ID, req.SourceHash, req.RuntimeVersion, req.PreludeVersion, now)
		if err != nil {
			return err
		}
		if n, _ := inserted.RowsAffected(); n == 1 {
			if _, err = tx.ExecContext(ctx, `INSERT INTO code_manifest VALUES(?,?,?,?,?,?,?)`, inv.ID, req.Source, `{}`, digest(`{}`), snapshots, digest(manifest), manifest); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO code_execution_state(execution_id,created_at,updated_at) VALUES(?,?,?)`, inv.ID, now, now); err != nil {
				return err
			}
		}
		var hash, source, runtime, prelude, frozenSnapshots string
		if err = tx.QueryRowContext(ctx, `SELECT m.manifest_hash,c.source_hash,c.runtime_version,c.prelude_version,m.snapshot_hash FROM code_manifest m JOIN code_execution c USING(execution_id) WHERE execution_id=?`, inv.ID).Scan(&hash, &source, &runtime, &prelude, &frozenSnapshots); err == sql.ErrNoRows {
			return tool.ErrReplay
		} else if err != nil {
			return err
		}
		drift = frozenSnapshots != snapshots || hash != digest(manifest) || source != req.SourceHash || runtime != req.RuntimeVersion || prelude != req.PreludeVersion
		req.ClaimToken = newID()
		res, err := tx.ExecContext(ctx, `UPDATE code_execution_state SET state='running',claim_token=?,owner_run_token=?,execution_epoch=execution_epoch+1,claim_expires_at=?,updated_at=?,version=version+1 WHERE execution_id=? AND desired_state='running' AND state IN ('running','suspended') AND (claim_token IS NULL OR claim_expires_at<=? OR owner_run_token!=?)`, req.ClaimToken, b.t.claim.(*claim).token, now+leaseDuration.Microseconds(), now, inv.ID, now, b.t.claim.(*claim).token)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return agent.ErrBusy
		}
		if err = tx.QueryRowContext(ctx, `SELECT execution_epoch FROM code_execution_state WHERE execution_id=?`, inv.ID).Scan(&req.ExecutionEpoch); err != nil {
			return err
		}
		return runEvent(ctx, tx, b.t.claim.RunID(), "code_execution.claimed", req.ExecutionEpoch)
	})
	if err == nil && drift {
		return tool.ErrReplay
	}
	return err
}

func (b *codeBridge) wake(ctx context.Context) (int64, error) {
	var seq int64
	err := b.t.runs.host.db.QueryRowContext(ctx, `SELECT wake_seq FROM code_execution_state WHERE execution_id=?`, b.req.ExecutionID).Scan(&seq)
	return seq, err
}

// release compares the observed wake in the same transaction that enters idle.
// A completion between the last check and release forces another drain.
func (b *codeBridge) release(ctx context.Context, observed int64) (bool, error) {
	idle := false
	err := b.t.runs.write(ctx, func(tx *sql.Tx) error {
		if err := b.fence(ctx, tx); err != nil {
			return err
		}
		var seq int64
		if err := tx.QueryRowContext(ctx, `SELECT wake_seq FROM code_execution_state WHERE execution_id=?`, b.req.ExecutionID).Scan(&seq); err != nil {
			return err
		}
		if seq != observed {
			return nil
		}
		now := time.Now().UnixMicro()
		if _, err := tx.ExecContext(ctx, `UPDATE code_execution_state SET state='suspended',processed_wake_seq=?,claim_token=NULL,owner_run_token=NULL,claim_expires_at=0,updated_at=?,version=version+1 WHERE execution_id=?`, seq, now, b.req.ExecutionID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE tool_invocation SET status='waiting',wait_reason='external_event',updated_at=?,version=version+1 WHERE invocation_id=? AND status='running'`, now, b.req.ExecutionID); err != nil {
			return err
		}
		idle = true
		return runEvent(ctx, tx, b.t.claim.RunID(), "code_execution.suspended", b.req.ExecutionEpoch)
	})
	return idle, err
}
func (b *codeBridge) abandon() error {
	return b.t.runs.write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE code_execution_state SET state='suspended',claim_token=NULL,owner_run_token=NULL,claim_expires_at=0,version=version+1 WHERE execution_id=? AND claim_token=? AND execution_epoch=?`, b.req.ExecutionID, b.req.ClaimToken, b.req.ExecutionEpoch)
		return err
	})
}

// Recovery is a scan of authority, not an independent scheduler. Run selection
// remains the existing Actor/Scheduler; Outbox messages are wake hints only.
func (r *Runs) recoverCode(ctx context.Context) error {
	return r.write(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT s.execution_id,i.run_id,s.execution_epoch FROM code_execution_state s JOIN tool_invocation i ON i.invocation_id=s.execution_id JOIN run r USING(run_id) WHERE s.desired_state='running' AND s.state IN ('running','suspended') AND r.status IN ('queued','running','waiting') AND ((s.claim_token IS NOT NULL AND s.claim_expires_at<=?) OR (s.claim_token IS NULL AND s.wake_seq>s.processed_wake_seq))`, time.Now().UnixMicro())
		if err != nil {
			return err
		}
		type item struct {
			id, run string
			epoch   int64
		}
		items := []item{}
		for rows.Next() {
			var v item
			if err = rows.Scan(&v.id, &v.run, &v.epoch); err != nil {
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
			if _, err = tx.ExecContext(ctx, `UPDATE code_execution_state SET state='suspended',claim_token=NULL,owner_run_token=NULL,claim_expires_at=0,processed_wake_seq=wake_seq,version=version+1 WHERE execution_id=?`, v.id); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE run SET status='running',wait_reason=NULL,updated_at=?,version=version+1 WHERE run_id=? AND status='waiting'`, time.Now().UnixMicro(), v.run); err != nil {
				return err
			}
			if err = runEvent(ctx, tx, v.run, "code_execution_ready", v.epoch); err != nil {
				return err
			}
		}
		return nil
	})
}

func (b *codeBridge) deliver(ctx context.Context, call string, reply codemode.Reply) (codemode.Reply, error) {
	err := b.t.runs.write(ctx, func(tx *sql.Tx) error {
		if err := b.fence(ctx, tx); err != nil {
			return err
		}
		var raw string
		err := tx.QueryRowContext(ctx, `SELECT sequence,reply_json FROM code_delivery WHERE execution_id=? AND call_id=?`, b.req.ExecutionID, call).Scan(&reply.DeliverySeq, &raw)
		if err == nil {
			return json.Unmarshal([]byte(raw), &reply)
		}
		if err != sql.ErrNoRows {
			return err
		}
		if err = tx.QueryRowContext(ctx, `SELECT coalesce(max(sequence),0)+1 FROM code_delivery WHERE execution_id=?`, b.req.ExecutionID).Scan(&reply.DeliverySeq); err != nil {
			return err
		}
		raw, _ = encoded(reply)
		_, err = tx.ExecContext(ctx, `INSERT INTO code_delivery VALUES(?,?,?,?)`, b.req.ExecutionID, call, reply.DeliverySeq, raw)
		return err
	})
	return reply, err
}
