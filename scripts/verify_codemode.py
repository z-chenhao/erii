#!/usr/bin/env python3
"""Real CLI -> ToolExecutor -> Node/Pi QuickJS -> nested ToolExecutor proof."""
import argparse
import json
from pathlib import Path
import shutil
import signal
import sqlite3
import tempfile

from verify_capabilities import CapabilityHarness

class CodeHarness(CapabilityHarness):
    def args(self, *args):
        return [str(self.binary), "-config", str(self.root / "config.yaml"), "-test-bridge", "codemode", *args]
from verify_main import require


def run(binary):
    report = {'version': 1, 'scope': 'QuickJS/ToolInvocation contract; full fixed-source kill/restart gates are verify_code_recovery.py',
              'checks': [], 'cleanup': False}
    with tempfile.TemporaryDirectory(prefix='erii-codemode-') as temp:
        root = Path(temp)
        node = shutil.which('node')
        require(node, 'doctor: Node required')
        script = Path(__file__).resolve().parents[1] / 'runtime/javascript/src/sidecar.mjs'
        require((script.parents[1] / 'node_modules/quickjs-wasi/quickjs.wasm').is_file(), 'doctor: npm ci --prefix runtime/javascript required')
        (root / 'config.yaml').write_text('version: 1\njavascript:\n  command: ' + json.dumps(node) + '\n  script: ' + json.dumps(str(script)) + '\n')
        h = CodeHarness(binary, root, fixture='codemode')
        h.setup()
        db_path = Path(h.env['ERII_DATA_ROOT']) / 'state.db'

        def execute(key, source, worker=False, timeout=320):
            plan = json.dumps([{'name': 'builtin__codemode__execute', 'arguments': {'source': source}}])
            h.send(key, ('Delegate capabilities: ' if worker else 'Capabilities: ') + plan)
            h.command('chat', 'work', timeout=timeout)
            with sqlite3.connect(db_path) as db:
                db.row_factory = sqlite3.Row
                rows = [dict(r) for r in db.execute('SELECT * FROM tool_invocation ORDER BY created_at,delivery_seq')]
            parents = [r for r in rows if r['tool_name'] == 'builtin__codemode__execute']
            parent = parents[-1]
            return parent, [r for r in rows if r['parent_invocation_id'] == parent['invocation_id']]

        try:
            # A wrapper coordinates Tools without owning their execution slot.
            # Bound this regression so starvation fails instead of spinning.
            config = root / 'config.yaml'
            default_config = config.read_text()
            config.write_text(default_config + 'tools:\n  run_concurrency: 1\n')
            p, nested = execute('single-slot', 'return await tools.builtin__text__echo({text:"nested"});', timeout=20)
            require(p['status'] == 'succeeded' and p['attempt_count'] == 1 and len(nested) == 1, f'single-slot wrapper starved: {p} {nested}')
            require(nested[0]['status'] == 'succeeded' and nested[0]['attempt_count'] == 1, f'single-slot Tool did not execute once: {nested}')
            require(json.loads(json.loads(p['result_json'])['text']) == [{'type': 'return', 'value': 'nested'}], 'single-slot output missing')
            report['checks'].append({'name': 'single-slot-nested-tool', 'run_concurrency': 1,
                                     'parent': p['invocation_id'], 'parent_attempts': p['attempt_count'],
                                     'nested': nested, 'output': json.loads(json.loads(p['result_json'])['text'])})
            p, nested = execute('single-slot-parallel', 'return await Promise.all([1,2,3].map(n=>tools.builtin__text__echo({text:"serial-"+n})));', timeout=20)
            require(p['status'] == 'succeeded' and len(nested) == 3 and all(i['status'] == 'succeeded' and i['attempt_count'] == 1 for i in nested), 'single-slot parallel calls lost or redispatched')
            output = json.loads(json.loads(p['result_json'])['text'])
            require(output == [{'type': 'return', 'value': ['serial-1', 'serial-2', 'serial-3']}], 'single-slot aggregation changed order')
            report['checks'].append({'name': 'single-slot-parallel-tools', 'run_concurrency': 1,
                                     'parent': p['invocation_id'], 'nested': nested, 'output': output})
            config.write_text(default_config)

            p, nested = execute('parallel', '''
const values = await Promise.all([1, 2, 3].map(n => tools.builtin__text__echo({text: "nested-" + n})));
console.log("HIDDEN_CONSOLE");
text(values.length);
return values;
''')
            require(p['status'] == 'succeeded', f'parent did not finish: {p}')
            output = json.loads(json.loads(p['result_json'])['text'])
            require(output == [{'type': 'text', 'text': '3'}, {'type': 'return', 'value': ['nested-1', 'nested-2', 'nested-3']}], f'wrong explicit output {output}')
            require(len(nested) == 3 and all(i['status'] == 'succeeded' for i in nested), 'nested invocation pipeline missing')
            require(all(i['replay_policy'] == 'safe' and i['attempt_count'] == 1 for i in nested), 'nested execution metadata')
            report['checks'].append({'name': 'parallel-explicit-output', 'parent': p['invocation_id'], 'nested': [i['invocation_id'] for i in nested], 'output': output})

            p, nested = execute('store-failure', 'store("durable", {count: 7}); text(load("durable")); throw new Error("fixture");')
            require(p['status'] == 'failed', 'throw did not fail')
            with sqlite3.connect(db_path) as db:
                value = db.execute('SELECT value_json FROM code_store WHERE execution_id=? AND key=?', (p['invocation_id'], 'durable')).fetchone()
            require(json.loads(value[0]) == {'count': 7}, 'Store lost before throw')
            p, _ = execute('store-isolated', 'return load("durable") === undefined;')
            require(json.loads(json.loads(p['result_json'])['text']) == [{'type': 'return', 'value': True}], 'Store escaped invocation scope')
            report['checks'].append({'name': 'synchronous-store-failure-and-isolation', 'value': json.loads(value[0])})

            p, nested = execute('cua', '''
const state = await cua.getState();
const app = await cua.getApp("notes");
const observation = await app.observe();
text(observation);
let rejected = false;
try { store("forbidden", {app}); } catch { rejected = true; }
return {computer: state.computerId, rejected, privateBridge: !ALL_TOOLS.some(t=>t.name.startsWith("builtin__computer__"))};
''')
            require(p['status'] == 'succeeded' and len(nested) == 3, f'CUA bridge missing {p} {nested}')
            output = json.loads(json.loads(p['result_json'])['text'])
            require(output[-1]['value'] == {'computer': 'fixture-computer', 'rejected': True, 'privateBridge': True}, 'Surface persisted or inventory wrong')
            report['checks'].append({'name': 'cua-discover-bind-invoke', 'nested': [i['tool_name'] for i in nested], 'output': output})

            p, _ = execute('main-exposure', 'return {agent: typeof agent, visible: ALL_TOOLS.some(t=>t.name === "builtin__agent__run"), tools: await searchTools("echo")};')
            output = json.loads(json.loads(p['result_json'])['text'])[-1]['value']
            require(output['agent'] == 'undefined' and output['visible'] is False and output['tools'][0]['name'] == 'builtin__text__echo', 'Main gained agent or discovery failed')
            report['checks'].append({'name': 'main-exposure-and-discovery', 'output': output})

            p, nested = execute('worker-exposure', 'return {agent: typeof agent.run, ns: await describeNamespace("agent")};', worker=True)
            require(p['status'] == 'succeeded', f'worker cell {p}')
            output = json.loads(json.loads(p['result_json'])['text'])[-1]['value']
            require(output['agent'] == 'function' and output['ns']['name'] == 'agent', f'Worker agent facade missing: {p} output={output}')
            report['checks'].append({'name': 'worker-exposure', 'output': output})
            p, nested = execute('child-actual', 'const child = await agent.run({task: "Capabilities: []"}); return {text: child.text, refs: child.artifactRefs};', worker=True)
            require(p['status'] == 'succeeded' and len(nested) == 1 and nested[0]['tool_name'] == 'builtin__agent__run', f'Agent Host bridge {p} {nested}')
            output = json.loads(json.loads(p['result_json'])['text'])[-1]['value']
            require(isinstance(output['text'], str) and output['refs'] == [], 'AgentResult contract')
            with sqlite3.connect(db_path) as db:
                child = db.execute('SELECT c.kind,c.worker_position,c.status,p.model_snapshot_id=c.model_snapshot_id FROM agent_invocation a JOIN run c ON c.run_id=a.child_run_id JOIN run p ON p.run_id=c.parent_run_id WHERE a.invocation_id=?', (nested[0]['invocation_id'],)).fetchone()
            require(child == ('worker', 'child', 'completed', 1), 'Child did not use inherited Worker/model')
            report['checks'].append({'name': 'agent-run-real-worker', 'output': output, 'child': child})

            # The effect stays unexecuted while its exact frozen action awaits approval.
            p, nested = execute('approval-deny', 'try { await tools.mcp__fixture__send({target:"denied",text:"never-send"}); } catch(e) { return e.code; }')
            require(p['status'] == 'waiting' and nested[0]['attempt_count'] == 0, 'effect dispatched before approval')
            pending = [a for a in h.command('approval', 'list') if a['status'] == 'pending']
            require(len(pending) == 1, 'expected one exact action approval')
            a = pending[0]
            h.command('approval', 'deny', a['approval_id'], a['frozen_arguments_hash'])
            h.command('chat', 'work')
            with sqlite3.connect(db_path) as db:
                final = db.execute('SELECT result_json,status FROM tool_invocation WHERE invocation_id=?', (p['invocation_id'],)).fetchone()
            require(final[1] == 'succeeded' and json.loads(json.loads(final[0])['text']) == [{'type':'return','value':'APPROVAL_DENIED'}], f'approval rejection not caught: {final}')
            require(not (Path(h.env['ERII_DATA_ROOT']) / 'effect-ledger.db').exists(), 'denied effect reached Provider')
            report['checks'].append({'name': 'exact-action-approval-rejection', 'attempt_count': nested[0]['attempt_count']})

            p, nested = execute('parallel-child-batch', 'const rows = await Promise.allSettled(Array.from({length:3},()=>agent.run({task:"Capabilities: []"}))); return rows.map(row=>row.status);', worker=True)
            require(p['status'] == 'succeeded' and len(nested) == 3 and all(c['status'] == 'succeeded' for c in nested), 'parallel Child calls lost at suspension')
            require(json.loads(json.loads(p['result_json'])['text']) == [{'type':'return','value':['fulfilled']*3}], 'parallel Child aggregation changed declaration order')
            with sqlite3.connect(db_path) as db:
                first = db.execute("SELECT attempt_id,sequence,message_json FROM code_event WHERE execution_id=? AND json_extract(message_json,'$.kind') IN ('execution_suspended','execution_completed') ORDER BY rowid LIMIT 1", (p['invocation_id'],)).fetchone()
                require(first is not None, 'batch emitted no authoritative outcome')
                finished = {json.loads(r[0])['callId'] for r in db.execute("SELECT message_json FROM code_event WHERE execution_id=? AND attempt_id=? AND sequence<=? AND json_extract(message_json,'$.kind')='call_finished'", (p['invocation_id'],first[0],first[1]))}
            initial = json.loads(first[2])['outcome']
            # A fast Child can already be terminal. Both finished and pending
            # Host calls prove admission; pending-only counts assume timing.
            accepted = finished | set(initial['pendingCallIds'])
            require(len(accepted) == 3, 'batch was serialized by premature VM suspension')
            report['checks'].append({'name':'parallel-child-batch-survives-suspension','nested':[c['invocation_id'] for c in nested], 'first_outcome':initial, 'initial_accepted_calls':sorted(accepted)})

            p, nested = execute('unawaited-effect', 'store("executions", (load("executions") || 0) + 1); tools.mcp__fixture__send({target:"unawaited",text:"registered"}); return "selected-output";')
            require(p['status'] == 'waiting' and len(nested) == 1, 'unawaited accepted effect was orphaned')
            a = next(a for a in h.command('approval', 'list') if a['status'] == 'pending')
            h.command('approval', 'approve', a['approval_id'], a['frozen_arguments_hash'])
            h.command('chat', 'work')
            with sqlite3.connect(db_path) as db:
                final = db.execute('SELECT result_json,status FROM tool_invocation WHERE invocation_id=?', (p['invocation_id'],)).fetchone()
                writes = db.execute('SELECT value_json FROM code_store WHERE execution_id=? AND key=?', (p['invocation_id'],'executions')).fetchone()[0]
                status = db.execute('SELECT status FROM tool_invocation WHERE invocation_id=?', (nested[0]['invocation_id'],)).fetchone()[0]
            require(final[1] == 'succeeded' and status == 'succeeded' and json.loads(writes) == 1, 'finished cell reexecuted or lost accepted call')
            require(json.loads(json.loads(final[0])['text']) == [{'type':'return','value':'selected-output'}], 'unawaited result leaked into finished output')
            report['checks'].append({'name':'unawaited-effect-drains-without-reexecuting-cell','store_executions':json.loads(writes),'output':json.loads(json.loads(final[0])['text'])})

            p, nested = execute('unawaited-child', 'agent.run({task:"Capabilities: []"}); return "child-started";', worker=True)
            require(p['status'] == 'succeeded' and len(nested) == 1 and nested[0]['status'] == 'succeeded', 'unawaited Worker orphaned')
            require(json.loads(json.loads(p['result_json'])['text']) == [{'type':'return','value':'child-started'}], 'unawaited Child result leaked')
            report['checks'].append({'name':'unawaited-child-completes-internally','status':nested[0]['status']})

            for boundary in ('before_provider_effect', 'after_provider_effect'):
                source = 'return await tools.mcp__fixture__send({target:' + json.dumps(boundary) + ',text:"exact-effect"});'
                p, nested = execute('crash-' + boundary, source)
                require(p['status'] == 'waiting' and len(nested) == 1, 'effect did not suspend for approval')
                approval = next(a for a in h.command('approval', 'list') if a['status'] == 'pending')
                h.command('approval', 'approve', approval['approval_id'], approval['frozen_arguments_hash'])
                process = h.hold(boundary)
                process.send_signal(signal.SIGKILL)
                process.communicate(timeout=10)
                with sqlite3.connect(db_path) as db:
                    before = db.execute('SELECT idempotency_key,status FROM tool_invocation WHERE invocation_id=?', (nested[0]['invocation_id'],)).fetchone()
                    calls_before = db.execute('SELECT count(*) FROM code_call WHERE execution_id=?', (p['invocation_id'],)).fetchone()[0]
                require(calls_before == 1, 'nested effect ran before CodeCall acceptance committed')
                h.command('chat', 'work')
                with sqlite3.connect(db_path) as db:
                    after = db.execute('SELECT idempotency_key,status,attempt_count FROM tool_invocation WHERE invocation_id=?', (nested[0]['invocation_id'],)).fetchone()
                    parent_status = db.execute('SELECT status FROM tool_invocation WHERE invocation_id=?', (p['invocation_id'],)).fetchone()[0]
                    count = db.execute('SELECT count(*) FROM code_call WHERE execution_id=?', (p['invocation_id'],)).fetchone()[0]
                with sqlite3.connect(Path(h.env['ERII_DATA_ROOT']) / 'effect-ledger.db') as db:
                    effects = db.execute('SELECT key,args FROM effects WHERE key=?', (before[0],)).fetchall()
                require(parent_status == 'succeeded' and after[0] == before[0] and after[1] == 'succeeded' and count == 1 and len(effects) == 1, 'nested effect recovery duplicated/lost identity or outcome')
                report['checks'].append({'name':'nested-effect-SIGKILL', 'boundary':boundary, 'before':before, 'after':after, 'effects':effects, 'code_calls':count})

            with sqlite3.connect(db_path) as db:
                report['events'] = [json.loads(r[0]) for r in db.execute('SELECT message_json FROM code_event ORDER BY execution_id,attempt_id,sequence')]
                report['schema_version'] = db.execute('PRAGMA user_version').fetchone()[0]
            require(report['events'] and report['schema_version'] == 17, 'durable events/migration missing')
        finally:
            h.cleanup()
    report['cleanup'] = not root.exists()
    require(report['cleanup'], 'disposable data retained')
    return report


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--evidence', type=Path, required=True)
    args = parser.parse_args()
    result = run(args.binary.resolve())
    args.evidence.parent.mkdir(parents=True, exist_ok=True)
    args.evidence.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps({'checks': len(result['checks']), 'cleanup': result['cleanup']}))
