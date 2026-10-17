-- Execution state is private to the ordinary Code Mode ToolInvocation. Source
-- and snapshots remain immutable in the manifest; leases never change them.
CREATE TABLE code_manifest (
 execution_id TEXT PRIMARY KEY REFERENCES code_execution(execution_id),
 source TEXT NOT NULL, input_json TEXT NOT NULL CHECK(json_valid(input_json)),
 input_hash TEXT NOT NULL, snapshot_hash TEXT NOT NULL, manifest_hash TEXT NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json))
) STRICT;
CREATE TRIGGER code_manifest_immutable BEFORE UPDATE ON code_manifest BEGIN SELECT RAISE(ABORT,'immutable code manifest'); END;
CREATE TRIGGER code_manifest_retained BEFORE DELETE ON code_manifest BEGIN SELECT RAISE(ABORT,'code manifest retained'); END;
CREATE TRIGGER code_manifest_replace BEFORE INSERT ON code_manifest WHEN EXISTS(SELECT 1 FROM code_manifest WHERE execution_id=NEW.execution_id) BEGIN SELECT RAISE(ABORT,'code manifest exists'); END;
CREATE TABLE code_execution_state (
 execution_id TEXT PRIMARY KEY REFERENCES code_execution(execution_id),
 state TEXT NOT NULL DEFAULT 'suspended' CHECK(state IN ('running','suspended','completed','failed','cancelled')),
 desired_state TEXT NOT NULL DEFAULT 'running' CHECK(desired_state IN ('running','paused','cancelled')),
 wake_seq INTEGER NOT NULL DEFAULT 0, processed_wake_seq INTEGER NOT NULL DEFAULT 0,
 execution_epoch INTEGER NOT NULL DEFAULT 0, claim_token TEXT, owner_run_token TEXT,
 claim_expires_at INTEGER NOT NULL DEFAULT 0, result_ref TEXT, error_json TEXT,
 replay_starts INTEGER NOT NULL DEFAULT 0, replay_hits INTEGER NOT NULL DEFAULT 0,
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, version INTEGER NOT NULL DEFAULT 1,
 CHECK(wake_seq>=processed_wake_seq AND processed_wake_seq>=0)
) STRICT;
CREATE INDEX code_recovery ON code_execution_state(state,claim_expires_at,wake_seq,processed_wake_seq);
CREATE TRIGGER code_state_version BEFORE UPDATE ON code_execution_state WHEN NEW.version!=OLD.version+1 OR NEW.execution_id!=OLD.execution_id OR NEW.created_at!=OLD.created_at OR NEW.wake_seq<OLD.wake_seq OR NEW.processed_wake_seq<OLD.processed_wake_seq OR NEW.execution_epoch<OLD.execution_epoch BEGIN SELECT RAISE(ABORT,'invalid code state update'); END;
-- call_id already has execution scope; v2 uses its trusted call key as suffix.
-- Keep prior v1 rows intact for diagnosis. They have no v2 manifest and cannot
-- silently replay under a different Runtime/Prelude.
CREATE VIEW code_call_state AS SELECT c.*,i.tool_name,i.status,i.result_ref,i.error_json,i.attempt_count,i.created_at,i.updated_at,a.child_run_id FROM code_call c JOIN tool_invocation i USING(invocation_id) LEFT JOIN agent_invocation a USING(invocation_id);
CREATE TABLE code_delivery (
 execution_id TEXT NOT NULL REFERENCES code_execution(execution_id), call_id TEXT NOT NULL,
 sequence INTEGER NOT NULL CHECK(sequence>0), reply_json TEXT NOT NULL CHECK(json_valid(reply_json)),
 PRIMARY KEY(execution_id,call_id), UNIQUE(execution_id,sequence)
) STRICT;
CREATE TRIGGER code_delivery_immutable BEFORE UPDATE ON code_delivery BEGIN SELECT RAISE(ABORT,'immutable code delivery'); END;
CREATE TRIGGER code_delivery_retained BEFORE DELETE ON code_delivery BEGIN SELECT RAISE(ABORT,'code delivery retained'); END;
-- Wake changes are in the same transaction as the authoritative transition.
CREATE TRIGGER code_tool_wake AFTER UPDATE OF status ON tool_invocation WHEN NEW.parent_invocation_id IS NOT NULL AND NEW.status!=OLD.status BEGIN
 UPDATE code_execution_state SET wake_seq=wake_seq+1,updated_at=NEW.updated_at,version=version+1 WHERE execution_id=NEW.parent_invocation_id;
END;
CREATE TRIGGER code_approval_wake AFTER UPDATE OF status ON approval BEGIN
 UPDATE code_execution_state SET wake_seq=wake_seq+1,version=version+1 WHERE execution_id=(SELECT parent_invocation_id FROM tool_invocation WHERE invocation_id=NEW.invocation_id);
END;
CREATE TRIGGER code_child_wake AFTER UPDATE OF status ON run WHEN NEW.status!=OLD.status BEGIN
 UPDATE code_execution_state SET wake_seq=wake_seq+1,updated_at=NEW.updated_at,version=version+1 WHERE execution_id IN (SELECT i.parent_invocation_id FROM agent_invocation a JOIN tool_invocation i USING(invocation_id) WHERE a.child_run_id=NEW.run_id);
END;
CREATE TRIGGER code_stop_intent AFTER UPDATE OF desired_state ON run WHEN NEW.desired_state!=OLD.desired_state BEGIN
 UPDATE code_execution_state SET desired_state=CASE WHEN NEW.desired_state='running' THEN 'running' WHEN NEW.desired_state='task_paused' THEN 'paused' ELSE 'cancelled' END,wake_seq=wake_seq+1,claim_token=NULL,owner_run_token=NULL,claim_expires_at=0,version=version+1 WHERE execution_id IN (SELECT invocation_id FROM tool_invocation WHERE run_id=NEW.run_id) AND state IN ('running','suspended');
END;
CREATE TRIGGER code_wrapper_terminal AFTER UPDATE OF status ON tool_invocation WHEN NEW.tool_name='builtin__codemode__execute' AND NEW.status IN ('succeeded','failed','cancelled','unknown') BEGIN
 UPDATE code_execution_state SET state=CASE WHEN NEW.status='succeeded' THEN 'completed' WHEN NEW.status='cancelled' THEN 'cancelled' ELSE 'failed' END,processed_wake_seq=wake_seq,result_ref=NEW.result_ref,error_json=NEW.error_json,claim_token=NULL,owner_run_token=NULL,claim_expires_at=0,updated_at=NEW.updated_at,version=version+1 WHERE execution_id=NEW.invocation_id;
END;

ALTER TABLE code_store ADD COLUMN value_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE code_store ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE code_call ADD COLUMN call_key TEXT GENERATED ALWAYS AS (substr(call_id,length(execution_id)+2)) VIRTUAL;
CREATE UNIQUE INDEX code_call_key ON code_call(execution_id,call_key);
CREATE TRIGGER code_instruction_wake AFTER INSERT ON task_instruction BEGIN
 UPDATE code_execution_state SET wake_seq=wake_seq+1,version=version+1 WHERE execution_id IN (SELECT i.invocation_id FROM tool_invocation i JOIN run r USING(run_id) WHERE r.task_id=NEW.task_id AND r.task_epoch=NEW.task_epoch) AND state IN ('running','suspended');
END;
CREATE TRIGGER code_input_wake AFTER INSERT ON main_input BEGIN
 UPDATE code_execution_state SET wake_seq=wake_seq+1,version=version+1 WHERE execution_id IN (SELECT invocation_id FROM tool_invocation WHERE run_id=NEW.run_id) AND state IN ('running','suspended');
END;
CREATE TRIGGER code_spend_wake AFTER UPDATE OF status ON spend_approval BEGIN
 UPDATE code_execution_state SET wake_seq=wake_seq+1,version=version+1 WHERE execution_id IN (SELECT i.invocation_id FROM tool_invocation i JOIN run r USING(run_id) WHERE coalesce(r.root_run_id,r.run_id)=NEW.root_run_id) AND state IN ('running','suspended');
END;
-- Store observations/writes are replayed too. Reading the last stored value
-- anew at an earlier source position would change deterministic control flow.
CREATE TABLE code_store_call (
 execution_id TEXT NOT NULL REFERENCES code_execution(execution_id), call_key TEXT NOT NULL,
 request_hash TEXT NOT NULL, reply_json TEXT NOT NULL CHECK(json_valid(reply_json)),
 PRIMARY KEY(execution_id,call_key)
) STRICT;
CREATE TRIGGER code_store_call_immutable BEFORE UPDATE ON code_store_call BEGIN SELECT RAISE(ABORT,'immutable store call'); END;
CREATE TRIGGER code_store_call_retained BEFORE DELETE ON code_store_call BEGIN SELECT RAISE(ABORT,'store call retained'); END;
