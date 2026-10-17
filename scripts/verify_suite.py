#!/usr/bin/env python3
"""Run the complete offline gate inventory, serially within each shard.

Shards own disjoint reports and disposable fixtures. Five-run gates are never
split across workers: one failure prevents that series from passing.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
import hashlib
import json
import os
from pathlib import Path
import platform
import signal
import subprocess
import sys
import threading
import time

ROOT = Path(__file__).resolve().parents[1]


@dataclass(frozen=True)
class Gate:
    name: str
    script: str
    report: str
    attempts: int = 1


# Complete baseline inventory, including the initial account check AND its
# five stability runs. No assertion, count, deadline or fault point changes.
SHARDS = {
    "runtime": (
        Gate("foundation", "verify.py", "verification"),
        Gate("main", "verify_main.py", "main"),
        Gate("tasks", "verify_tasks.py", "tasks"),
        Gate("children", "verify_children.py", "children", 5),
        Gate("scheduling", "verify_scheduling.py", "scheduling", 5),
        Gate("spend", "verify_spend.py", "spend", 5),
        Gate("task-control", "verify_task_control.py", "task-control", 5),
        Gate("cases", "run_cases.py", "cases"),
        Gate("task-cases", "run_task_cases.py", "task-cases"),
        Gate("events", "", "events"),
    ),
    "codemode": (Gate("codemode", "verify_codemode.py", "codemode", 5),
                 Gate("code-recovery", "verify_code_recovery.py", "code-recovery", 5)),
    "security": (
        Gate("tools", "verify_tools.py", "tools", 5),
        Gate("artifacts", "verify_artifacts.py", "artifacts", 5),
    ),
    "delivery": (Gate("delivery", "verify_delivery.py", "delivery", 5),),
    "providers": (
        Gate("local-provider", "verify_local_provider.py", "local-provider"),
        Gate("account-initial", "verify_account.py", "account"),
        Gate("capabilities", "verify_capabilities.py", "capabilities", 5),
        Gate("tui", "verify_tui.py", "tui", 5),
        Gate("terminal-input", "verify_terminal_input.py", "terminal-input"),
        Gate("pi-bridge", "", "pi-bridge", 5),
        Gate("account", "verify_account.py", "account", 5),
        Gate("models", "", "models"),
    ),
}


def report_name(gate, attempt):
    suffix = f"-{attempt}" if gate.attempts > 1 else ""
    if gate.name == "tui" and attempt == 1:
        suffix = ""  # Preserve the existing feature-map path.
    return gate.report + suffix + ".json"


def command(gate, attempt, binary, evidence):
    env = os.environ.copy()
    if gate.script:
        env["ERII_PROFILE_DIR"] = str(evidence / "profiles" / f"{gate.name}-{attempt}")
        args = [sys.executable, "scripts/profile_harness.py", "scripts/" + gate.script,
                "--binary", str(binary), "--evidence", str(evidence / report_name(gate, attempt))]
        output = evidence / "logs" / f"{gate.name}-{attempt}.log"
    elif gate.name == "pi-bridge":
        args = ["node", "--test", "--test-reporter=tap",
                *map(str, sorted(Path("provider-bridge/test").glob("*.test.mjs")))]
        output = evidence / "pi-bridge.tap"
    elif gate.name == "models":
        args = ["go", "test", "-count=5", "-json", "./internal/llm/...", "./internal/agent/sqlite"]
        output = evidence / "models.jsonl"
    else:
        args = ["go", "test", "-count=1", "-v", "./internal/event/integration"]
        env.update(ERII_TEST_BINARY=str(binary), ERII_EVENT_EVIDENCE=str(evidence / "events.json"))
        output = evidence / "logs" / "events-1.log"
    return args, env, output


def run_command(args, env, output, stopped):
    """Reap owned groups on interruption; Python harnesses unwind cleanup."""
    if stopped.is_set():
        return 130
    with output.open("a") as log:
        with subprocess.Popen(args, cwd=ROOT, env=env, stdout=log,
                              stderr=subprocess.STDOUT, start_new_session=True) as child:
            while True:
                try:
                    return child.wait(timeout=.2)
                except subprocess.TimeoutExpired:
                    if stopped.is_set():
                        os.killpg(child.pid, signal.SIGTERM)
                        try:
                            return child.wait(timeout=10)
                        except subprocess.TimeoutExpired:
                            os.killpg(child.pid, signal.SIGKILL)
                            return child.wait()


def run_shard(name, binary, evidence, stopped, gates=None, previous=()):
    rows = list(previous)
    for gate in SHARDS[name] if gates is None else gates:
        for attempt in range(1, gate.attempts + 1):
            args, env, output = command(gate, attempt, binary, evidence)
            started = time.monotonic()
            try:
                code = run_command(args, env, output, stopped)
            except OSError as error:
                output.write_text(str(error) + "\n")
                code = 127
            row = dict(gate=gate.name, attempt=attempt, command=args, returncode=code,
                       elapsed_seconds=time.monotonic() - started, log=str(output))
            rows.append(row)
            (evidence / f"timings-{name}.json").write_text(json.dumps(rows, indent=2) + "\n")
            print(f"[{name}] {gate.name} {attempt}/{gate.attempts}: {row['elapsed_seconds']:.2f}s (exit {code})", flush=True)
            if code:
                if output.exists():
                    print(output.read_text()[-8000:], file=sys.stderr)
                return rows
    return rows


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--shard", choices=SHARDS)
    parser.add_argument("--jobs", type=int, default=2)
    parser.add_argument("--binary", type=Path, default=Path("bin/personal-agent"))
    parser.add_argument("--evidence", type=Path, default=Path("evidence"))
    args = parser.parse_args()
    if args.jobs < 1:
        parser.error("--jobs must be positive")
    os.chdir(ROOT)
    binary, evidence = args.binary.resolve(), args.evidence.resolve()
    if not binary.is_file():
        parser.error("binary missing; run make build")
    names = [args.shard] if args.shard else list(SHARDS)
    evidence.mkdir(parents=True, exist_ok=True)
    (evidence / "logs").mkdir(exist_ok=True)
    # Delete only reports owned by the selected shards; never user state.
    for name in names:
        (evidence / f"timings-{name}.json").unlink(missing_ok=True)
        for gate in SHARDS[name]:
            for attempt in range(1, gate.attempts + 1):
                _, _, output = command(gate, attempt, binary, evidence)
                output.unlink(missing_ok=True)
                (evidence / report_name(gate, attempt)).unlink(missing_ok=True)
                if gate.script:
                    (evidence / "profiles" / f"{gate.name}-{attempt}" /
                     (Path(gate.script).stem + ".json")).unlink(missing_ok=True)
        if name == "runtime":
            (evidence / "relay.json").unlink(missing_ok=True)
    stopped = threading.Event()
    for sig in (signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, lambda *_: stopped.set())
    started = time.monotonic()
    plans = {name: SHARDS[name] for name in names}
    # Repeated Go conformance contains real 100 ms recovery deadlines. Keep it
    # isolated from unrelated local harness I/O, as it is on its CI runner.
    deferred = None
    if not args.shard and "providers" in plans:
        deferred = next((g for g in plans["providers"] if g.name == "models"), None)
        plans["providers"] = tuple(g for g in plans["providers"] if g != deferred)
    with ThreadPoolExecutor(max_workers=args.jobs) as pool:
        results = list(pool.map(lambda name: run_shard(name, binary, evidence, stopped, plans[name]), names))
    if deferred is not None and not stopped.is_set():
        index = names.index("providers")
        prefix = results[index]
        if len(prefix) == sum(g.attempts for g in plans["providers"]) and all(r["returncode"] == 0 for r in prefix):
            results[index] = run_shard("providers", binary, evidence, stopped, (deferred,), prefix)
    passed = not stopped.is_set() and all(
        len(rows) == sum(g.attempts for g in SHARDS[name]) and all(r["returncode"] == 0 for r in rows)
        for name, rows in zip(names, results))
    report = dict(version=1, source_sha=subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
                  workflow_sha=os.environ.get("GITHUB_SHA"), platform=platform.platform(), jobs=args.jobs,
                  binary_sha256=hashlib.sha256(binary.read_bytes()).hexdigest(),
                  elapsed_seconds=time.monotonic() - started, passed=passed,
                  shards=dict(zip(names, results)))
    (evidence / ("suite-" + (args.shard or "all") + ".json")).write_text(json.dumps(report, indent=2) + "\n")
    return 0 if passed else 1


if __name__ == "__main__":
    sys.exit(main())
