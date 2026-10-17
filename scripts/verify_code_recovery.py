#!/usr/bin/env python3
"""Fixed-source recovery through real CLI, QuickJS, Worker and Tool boundaries."""
import argparse
from collections import Counter
import json
from pathlib import Path
import shutil
import signal
import sqlite3
import subprocess
import tempfile
import time

from verify_codemode import CodeHarness
from verify_main import require


def read(db_path, query, args=()):
    with sqlite3.connect(db_path) as db:
        db.row_factory = sqlite3.Row
        return [dict(r) for r in db.execute(query, args)]


def run(binary):
    report = {'version': 1, 'checks': [], 'cleanup': False}
    with tempfile.TemporaryDirectory(prefix='erii-code-recovery-') as temp:
        root = Path(temp)
        node = shutil.which('node')
        script = Path(__file__).resolve().parents[1] / 'runtime/javascript/src/sidecar.mjs'
        require(node and (script.parents[1] / 'node_modules/quickjs-wasi/quickjs.wasm').is_file(), 'doctor: pinned Node/QuickJS required')
        for variant in ('sidecar', 'go', 'shutdown'):
            work = root / variant
            work.mkdir()
            (work / 'config.yaml').write_text('version: 1\njavascript:\n  command: ' + json.dumps(node) + '\n  script: ' + json.dumps(str(script)) + '\n')
            h = CodeHarness(binary, work, fixture='codemode')
            h.setup()
            state = Path(h.env['ERII_DATA_ROOT']) / 'state.db'
            release = work / 'release'
            try:
                # First 37 provider calls finish. Eight accepted Workers then
                # wait for exact effect approvals. Remaining 55 are not issued.
                child = 'Capabilities: ' + json.dumps([{'name': 'mcp__fixture__send', 'arguments': {'target': 'recovery', 'text': 'approved-child'}}])
                source = '''
const first = await Promise.all(Array.from({length:37}, (_,i)=>agent.run({task:"Capabilities: []"})));
const active = await Promise.all(Array.from({length:8}, (_,i)=>agent.run({task:CHILD})));
const rest = await Promise.all(Array.from({length:55}, (_,i)=>agent.run({task:"Capabilities: []"})));
return {count:first.length+active.length+rest.length};
'''.replace('CHILD', json.dumps(child))
                plan = json.dumps([{'name': 'builtin__codemode__execute', 'arguments': {'source': source}}])
                h.send('recover-' + variant, 'Delegate capabilities: ' + plan)
                start = time.monotonic()
                process = h.hold('code_37_completed_8_active', extra_flags=('-test-main-release', str(release)))
                before = read(state, "SELECT c.call_id,c.invocation_id,a.child_run_id,r.status FROM code_call c JOIN agent_invocation a USING(invocation_id) JOIN run r ON r.run_id=a.child_run_id")
                require(len(before) == 45 and sum(r['status'] == 'completed' for r in before) == 37, 'not actual 37 completed/8 active')
                completed = [r['child_run_id'] for r in before if r['status'] == 'completed']
                generations = read(state, "SELECT generation_id,run_id FROM model_generation WHERE status='completed'")
                done_generations = [g['generation_id'] for g in generations if g['run_id'] in completed]
                require(len(done_generations) == 37, 'completed work did not use exactly 37 provider calls')
                # The Fake peer owns an independent append-only dispatch audit.
                audit_path = Path(h.env['ERII_DATA_ROOT']) / 'fake-provider-calls.jsonl'
                audit = Counter(json.loads(line)['generation_id'] for line in audit_path.read_text().splitlines())
                require(all(audit[g] == 1 for g in done_generations), 'provider dispatched completed work more than once before kill')
                sidecar_pid = None
                if variant == 'sidecar':
                    children = subprocess.check_output(['ps', '-axo', 'pid=,ppid=,command='], text=True)
                    matches = [int(p[0]) for line in children.splitlines() if len(p := line.split(None, 2)) == 3 and int(p[1]) == process.pid and p[2].endswith(str(script))]
                    require(len(matches) == 1, 'must identify exact owned Sidecar PID')
                    sidecar_pid = matches[0]
                    # Node already disposed its Worker; kill its actual process
                    # while Go is awaiting the durable suspension acknowledgement.
                    import os
                    os.kill(sidecar_pid, signal.SIGKILL)
                    release.touch()
                    stdout, stderr = process.communicate(timeout=60)
                    require(process.returncode == 0, f'Sidecar recovery failed: {stdout} {stderr}')
                elif variant == 'shutdown':
                    process.send_signal(signal.SIGTERM)
                    stdout, stderr = process.communicate(timeout=15)
                    require(process.returncode == 0, f'shutdown failed: {stdout} {stderr}')
                else:
                    process.send_signal(signal.SIGKILL)
                    process.communicate(timeout=10)
                    require(process.returncode == -signal.SIGKILL, 'Go was not SIGKILLed')
                h.command('chat', 'work', timeout=90)
                held = read(state, "SELECT * FROM code_execution_state")
                require(len(held) == 1 and held[0]['state'] == 'suspended' and held[0]['claim_token'] is None, 'approval retained execution lease')
                suspended = read(state, "SELECT message_json FROM code_event WHERE json_extract(message_json,'$.kind')='execution_suspended'")
                metrics = [json.loads(r['message_json'])['metrics'] for r in suspended]
                require(metrics and all(m['activeVMs'] == 0 for m in metrics), 'suspended approval retained VM')
                pending = [a for a in h.command('approval', 'list') if a['status'] == 'pending']
                require(len(pending) == 8, 'eight active calls lost their approvals')
                for a in pending:
                    h.command('approval', 'approve', a['approval_id'], a['frozen_arguments_hash'])
                h.command('chat', 'work', timeout=90)
                after = read(state, "SELECT c.call_id,c.invocation_id,a.child_run_id,r.status FROM code_call c JOIN agent_invocation a USING(invocation_id) JOIN run r ON r.run_id=a.child_run_id")
                require(len(after) == 100 and all(r['status'] == 'completed' for r in after), '100 calls did not finish')
                by_key = {r['call_id']: r for r in after}
                require(all(by_key[r['call_id']]['invocation_id'] == r['invocation_id'] and by_key[r['call_id']]['child_run_id'] == r['child_run_id'] for r in before), 'accepted work did not reattach')
                audit = Counter(json.loads(line)['generation_id'] for line in audit_path.read_text().splitlines())
                require(all(audit[g] == 1 for g in done_generations), 'recovery redispatched completed provider calls')
                final = read(state, "SELECT * FROM code_execution_state")[0]
                parent = read(state, "SELECT result_json,status FROM tool_invocation WHERE tool_name='builtin__codemode__execute'")[0]
                require(parent['status'] == 'succeeded' and json.loads(json.loads(parent['result_json'])['text']) == [{'type':'return','value':{'count':100}}], 'aggregation changed')
                require(final['state'] == 'completed' and final['claim_token'] is None, 'terminal lease leaked')
                require(read(state, 'SELECT descendants FROM run_tree')[0]['descendants'] == 100, 'replay consumed descendant quota')
                report['checks'].append({'variant':variant, 'signal':'SIGTERM' if variant == 'shutdown' else 'SIGKILL','sidecar_pid':sidecar_pid,'before':before,'after':after,'completed_provider_dispatch_counts':{g:audit[g] for g in done_generations},'execution':final,'suspended_memory_metrics':metrics,'elapsed_seconds':time.monotonic()-start})
                # Cancellation reaches a live Root/Child tree through Main's
                # public Task tool; no test directly writes domain stop intent.
                cancel_source = 'return await agent.run({task:' + json.dumps(child) + '});'
                cancel_plan = json.dumps([{'name':'builtin__codemode__execute','arguments':{'source':cancel_source}}])
                h.send('cancel-start', 'Delegate capabilities: ' + cancel_plan)
                h.command('chat', 'work')
                task = read(state, "SELECT task_id,task_ref FROM task WHERE status='active'")[0]
                stops = json.dumps([{'name':'builtin__task__manage','arguments':{'operation':'cancel','taskRef':task['task_ref']}}])
                h.send('cancel-control', 'Capabilities: ' + stops)
                h.command('chat', 'work')
                cancelled = read(state, "SELECT run_id,status,desired_state FROM run WHERE task_id=?", (task['task_id'],))
                require(len(cancelled) == 2 and all(r['status'] == 'cancelled' for r in cancelled), 'cancel did not settle entire Code Mode tree')
                execution = read(state, "SELECT s.* FROM code_execution_state s JOIN tool_invocation i ON i.invocation_id=s.execution_id JOIN run r USING(run_id) WHERE r.task_id=?", (task['task_id'],))[0]
                require(execution['state'] == 'cancelled' and execution['desired_state'] == 'cancelled' and execution['claim_token'] is None, 'cancel left execution live')
                h.command('chat', 'work')
                require(not read(state, "SELECT 1 FROM tool_invocation i JOIN run r USING(run_id) WHERE r.task_id=? AND i.status IN ('queued','running','waiting')", (task['task_id'],)), 'restart revived cancelled nested work')
                report['checks'][-1]['cancelled_tree'] = cancelled
            finally:
                h.cleanup()
    report['cleanup'] = not root.exists()
    require(report['cleanup'], 'disposable state leaked')
    return report


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--evidence', type=Path, required=True)
    args = parser.parse_args()
    result = run(args.binary.resolve())
    args.evidence.parent.mkdir(parents=True, exist_ok=True)
    args.evidence.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps({'checks':len(result['checks']),'cleanup':result['cleanup']}))
