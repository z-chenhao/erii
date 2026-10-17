"""Regression protection for gate completeness, failures and cancellation."""
import itertools
import json
import os
from pathlib import Path
import re
import signal
import sys
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

import ci_gate
import verify_suite as suite


class VerificationTests(unittest.TestCase):
    def test_pre_optimization_inventory_and_disjoint_evidence(self):
        expected = {
            "verify.py": 1, "verify_local_provider.py": 1, "verify_account.py": 6,
            "verify_main.py": 1, "verify_tasks.py": 1, "verify_children.py": 5,
            "verify_scheduling.py": 5, "verify_spend.py": 5, "verify_task_control.py": 5,
            "verify_tools.py": 5, "verify_artifacts.py": 5, "verify_delivery.py": 5,
            "verify_capabilities.py": 5, "run_cases.py": 1, "run_task_cases.py": 1,
            "verify_code_recovery.py": 5, "verify_tui.py": 5, "verify_terminal_input.py": 1, "verify_codemode.py": 5,
        }
        counts, paths = {}, set()
        gates = [gate for group in suite.SHARDS.values() for gate in group]
        for gate in gates:
            if gate.script:
                counts[gate.script] = counts.get(gate.script, 0) + gate.attempts
                for attempt in range(1, gate.attempts + 1):
                    path = suite.report_name(gate, attempt)
                    self.assertNotIn(path, paths)
                    paths.add(path)
        self.assertEqual(counts, expected)
        scripts = suite.ROOT / "scripts"
        discovered = {p.name for p in scripts.glob("verify*.py")} - {"verify_suite.py"}
        discovered.update(p.name for p in scripts.glob("run*cases.py"))
        self.assertEqual(set(counts), discovered, "every standalone harness must be registered")
        self.assertEqual([(g.name, g.attempts) for g in gates if not g.script],
                         [("events", 1), ("pi-bridge", 5), ("models", 1)])
        model = next(g for g in gates if g.name == "models")
        args, _, _ = suite.command(model, 1, Path('/binary'), Path('/evidence'))
        self.assertEqual(args, ['go', 'test', '-count=5', '-json', './internal/llm/...', './internal/agent/sqlite'])

    def test_aggregate_rejects_every_non_success_combination(self):
        for states in itertools.product(['success', 'failure', 'cancelled', 'skipped'], repeat=3):
            needs = {name: {'result': value} for name, value in zip(sorted(ci_gate.REQUIRED), states)}
            self.assertEqual(ci_gate.passed(needs), all(s == 'success' for s in states))
        good = {name: {'result': 'success'} for name in ci_gate.REQUIRED}
        for name in good:
            self.assertFalse(ci_gate.passed({k: v for k, v in good.items() if k != name}))
        for invalid in [None, [], {}, dict(good, unexpected={'result': 'success'}),
                        dict(good, race={}), dict(good, race='success')]:
            self.assertFalse(ci_gate.passed(invalid))

    def test_workflow_topology_is_non_bypassable(self):
        workflow = (suite.ROOT / '.github/workflows/ci.yml').read_text()
        shards = json.loads(re.search(r'        shard: (\[.*\])', workflow)[1])
        self.assertEqual(set(shards), set(suite.SHARDS))
        self.assertEqual(len(shards), len(suite.SHARDS))
        needs = json.loads(re.search(r'    needs: (\[.*\])', workflow)[1])
        self.assertEqual(set(needs), ci_gate.REQUIRED)
        self.assertIn('name: Ubuntu checks', workflow)
        self.assertIn('if: ${{ always() }}', workflow)
        self.assertIn('NEEDS_JSON: ${{ toJSON(needs) }}', workflow)
        self.assertIn('run: python3 scripts/ci_gate.py', workflow)
        self.assertNotIn('continue-on-error', workflow)
        self.assertNotIn('paths-ignore:', workflow)
        self.assertIn('cancel-in-progress: true', workflow)
        self.assertIn('fail-fast: false', workflow)
        self.assertIn('retention-days: 14', workflow)
        self.assertIn('if-no-files-found: error', workflow)
        self.assertEqual(workflow.count('ref: ${{ github.event.pull_request.head.sha || github.sha }}'), 4)

    def test_failed_repetition_stops_and_records_failure(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / 'logs').mkdir()
            gate = suite.Gate('test', 'fixture.py', 'test', 5)
            def command(*_):
                return ([sys.executable, '-c', 'print("observed failure"); raise SystemExit(9)'],
                        os.environ.copy(), root / 'logs/test.log')
            handlers = {sig: signal.getsignal(sig) for sig in [signal.SIGINT, signal.SIGTERM]}
            try:
                with patch.dict(suite.SHARDS, {'test': (gate,)}, clear=True), patch.object(suite, 'command', command), \
                     patch.object(sys, 'argv', ['verify_suite.py', '--binary', sys.executable, '--evidence', str(root)]):
                    self.assertEqual(suite.main(), 1)
            finally:
                for sig, handler in handlers.items():
                    signal.signal(sig, handler)
            report = json.loads((root / 'suite-all.json').read_text())
            self.assertFalse(report['passed'])
            self.assertEqual(len(report['shards']['test']), 1)
            self.assertEqual(json.loads((root / 'timings-test.json').read_text())[0]['returncode'], 9)

    def test_cancellation_reaps_owned_process_and_runs_finally(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            script = root / 'fixture.py'
            ready, cleaned = root / 'ready', root / 'cleaned'
            script.write_text('from pathlib import Path\nimport time\n'
                              'def scenario(name):\n try:\n'
                              f'  Path({str(ready)!r}).touch()\n  time.sleep(60)\n finally:\n'
                              f'  Path({str(cleaned)!r}).touch()\nscenario("owned-case")\n')
            stopped = threading.Event()
            def interrupt():
                deadline = time.monotonic() + 5
                while not ready.exists() and time.monotonic() < deadline:
                    time.sleep(.01)
                stopped.set()
            thread = threading.Thread(target=interrupt)
            thread.start()
            env = dict(os.environ, ERII_PROFILE_DIR=str(root / 'profile'))
            try:
                code = suite.run_command([sys.executable, 'scripts/profile_harness.py', str(script)],
                                         env, root / 'log', stopped)
            finally:
                thread.join(timeout=6)
            self.assertNotEqual(code, 0)
            self.assertTrue(ready.exists())
            self.assertTrue(cleaned.exists())
            profile = json.loads((root / 'profile/fixture.json').read_text())
            self.assertGreater(profile['elapsed_seconds'], 0)
            self.assertEqual([c['name'] for c in profile['cases']], ['owned-case'])

    def test_full_suite_isolates_models_after_other_harnesses(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            trace = root / 'order'
            gates = {'runtime': (suite.Gate('slow', '', 'slow'),),
                     'providers': (suite.Gate('fast', '', 'fast'), suite.Gate('models', '', 'models'))}
            def command(gate, *_):
                source = ('import time\nfrom pathlib import Path\n'
                          f'p=Path({str(trace)!r})\n')
                if gate.name == 'models':
                    source += 'assert set(p.read_text().splitlines()) == {"slow", "fast"}\n'
                else:
                    source += f'time.sleep({.3 if gate.name == "slow" else .01})\n'
                    source += f'with p.open("a") as f: f.write({(gate.name + chr(10))!r})\n'
                return [sys.executable, '-c', source], os.environ.copy(), root / 'logs' / (gate.name + '.log')
            handlers = {sig: signal.getsignal(sig) for sig in [signal.SIGINT, signal.SIGTERM]}
            try:
                with patch.dict(suite.SHARDS, gates, clear=True), patch.object(suite, 'command', command), \
                     patch.object(sys, 'argv', ['verify_suite.py', '--binary', sys.executable, '--evidence', str(root)]):
                    self.assertEqual(suite.main(), 0)
            finally:
                for sig, handler in handlers.items():
                    signal.signal(sig, handler)
            result = json.loads((root / 'suite-all.json').read_text())
            self.assertTrue(result['passed'])
            self.assertEqual([r['gate'] for r in result['shards']['providers']], ['fast', 'models'])


if __name__ == '__main__':
    unittest.main()
