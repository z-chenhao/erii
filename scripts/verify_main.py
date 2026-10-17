#!/usr/bin/env python3
"""Launch, doctor, drive, read back and clean up the actual Main CLI.

No online services. SIGKILL points are opt-in Host hooks, reached by the same
Actor/Loop/Tool/Conversation path as an ordinary configured provider.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
import os
from pathlib import Path
import selectors
import signal
import sqlite3
import subprocess
import tempfile
import time


def require(value, message):
    if not value:
        raise RuntimeError(message)


class MainHarness:
    def __init__(self, binary, root, fixture="main"):
        self.binary = binary
        self.root = root
        self.env = {k: v for k, v in os.environ.items() if not k.startswith("ERII_")}
        self.env["ERII_DATA_ROOT"] = str(root / "data")
        self.children = []
        self.fixture = fixture

    def args(self, *args):
        return [str(self.binary), "-test-bridge", self.fixture, *args]

    def command(self, *args, text=None, ok=True):
        result = subprocess.run(self.args(*args), input=text, text=True, capture_output=True,
                                env=self.env, cwd=self.root, timeout=20)
        require(not ok or result.returncode == 0, f"{args}: {result.stderr}")
        return json.loads(result.stdout) if ok else result

    def setup(self, model="fixture-model", metadata=None):
        status = self.command("status")
        profile = (self.command("model", "add", text=json.dumps(metadata)) if metadata is not None else
                   self.command("model", "add", "http://127.0.0.1:1/v1", model))
        self.command("model", "test", profile["profile_id"])
        status = self.command("model", "select", profile["profile_id"], str(profile["version"]),
                              str(status["config_version"]))
        require(status["agent_ingress_enabled"], "doctor: selected fixture not ready")
        return profile["profile_id"]

    def send(self, key, text):
        return self.command("chat", "send", key, text=text)

    def wait_state(self, predicate, description, reader=None, timeout=10):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            value = reader(self) if reader else self.state()
            if predicate(value):
                return value
            time.sleep(0.025)
        raise RuntimeError("state boundary not reached: " + description)

    def hold(self, boundary, extra_flags=()):
        child = subprocess.Popen(self.args("-test-main-hold", boundary, *extra_flags, "chat", "work"),
                                 stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
                                 env=self.env, cwd=self.root)
        self.children.append(child)
        with selectors.DefaultSelector() as selector:
            selector.register(child.stdout, selectors.EVENT_READ)
            require(selector.select(15), f"boundary never reached: {boundary}")
        line = child.stdout.readline()
        require(line and json.loads(line).get("boundary") == boundary,
                f"exited before boundary {boundary}: {line} {child.stderr.read() if not line else ''}")
        return child

    def state(self):
        result = {}
        with sqlite3.connect(Path(self.env["ERII_DATA_ROOT"]) / "state.db") as db:
            db.row_factory = sqlite3.Row
            for name, query in {
                "runs": "SELECT run_id,kind,task_id,parent_run_id,root_run_id,task_epoch,depth,status,desired_state,model_snapshot_id,capability_snapshot_id,checkpoint_json,error_code,version FROM run ORDER BY created_at",
                "generations": "SELECT generation_id,run_id,step_index,request_hash,request_json,status,result_json,error_code,attempt_count FROM model_generation ORDER BY created_at",
                "tools": "SELECT invocation_id,run_id,provider_exchange_id,provider_call_id,tool_name,arguments_json,request_hash,replay_policy,status,result_json,delivery_seq,attempt_count FROM tool_invocation ORDER BY created_at,delivery_seq",
                "deliveries": "SELECT delivery_id,message_id,status,idempotency_key FROM delivery_request ORDER BY created_at",
                "usage": "SELECT u.generation_id,u.snapshot_id,u.error_code FROM provider_usage u JOIN model_generation g USING(generation_id) ORDER BY g.created_at",
                "snapshots": "SELECT * FROM model_snapshot ORDER BY created_at",
                "capabilities": "SELECT * FROM capability_snapshot ORDER BY created_at",
                "inputs": "SELECT sequence,request_key,message_json FROM main_input ORDER BY sequence",
                "trace": "SELECT run_id,sequence,event_json,occurred_at FROM run_trace ORDER BY run_id,sequence",
            }.items():
                result[name] = [dict(row) for row in db.execute(query)]
        result["conversation"] = self.command("chat", "history")
        return result

    def cleanup(self):
        for child in self.children:
            if child.poll() is None:
                child.kill()
            child.communicate(timeout=10)


def assert_final(state, users, replies=1):
    messages = state["conversation"]
    require(len(messages) == users + replies, "duplicate or missing semantic messages")
    require([m["sequence"] for m in messages] == list(range(1, len(messages) + 1)), "message order")
    require(len({m["message_id"] for m in messages}) == len(messages), "duplicate semantic IDs")
    require(sum(m["role"] == "user" for m in messages) == users, "user message lost")
    require(sum(m["role"] == "assistant" for m in messages) == replies, "assistant count")
    require(all(p["type"] == "text" for m in messages for p in m["content"]), "Tool trace leaked to history")
    require(all(r["status"] == "completed" for r in state["runs"]), "Run failed or stayed nonterminal")
    require(len(state["deliveries"]) == replies, "delivery intent duplicated/missing")
    require(all(g["status"] == "completed" for g in state["generations"]), "generation incomplete")
    require(len(state["usage"]) == len(state["generations"]), "completion and usage not atomic")
    require(all(t["status"] == "succeeded" for t in state["tools"]), "Tool failed/incomplete")


def run(binary):
    report = {"version": 1, "scope": "M1 Main slice; no M3 Security or M5 JavaScript replay claim",
              "checks": [], "crashes": []}
    with tempfile.TemporaryDirectory(prefix="erii-main-") as temp:
        root = Path(temp)

        def scenario(name, action):
            folder = root / name
            folder.mkdir()
            h = MainHarness(binary, folder)
            try:
                return action(h)
            finally:
                h.cleanup()

        def concurrent(h):
            rejected = h.command("chat", "send", "unconfigured", text="hello", ok=False)
            require(rejected.returncode != 0 and "setup_required" in rejected.stderr, "setup ingress opened")
            h.setup()
            with ThreadPoolExecutor(max_workers=8) as executor:
                receipts = list(executor.map(lambda i: h.send(f"input-{i}", f"message {i}"), range(16)))
            require(len({r["run_id"] for r in receipts}) == 1, "concurrent ingress created multiple Main Runs")
            require(h.send("input-0", "message 0") == receipts[0], "ingress retry not idempotent")
            unconfigured = subprocess.run([str(h.binary), "chat", "work"], text=True, capture_output=True,
                                          env=h.env, cwd=h.root, timeout=10)
            require(unconfigured.returncode != 0 and "setup_required" in unconfigured.stderr and "panic" not in unconfigured.stderr,
                    "restart without the explicitly configured Bridge must fail closed")
            drift = h.command("chat", "send", "input-0", text="changed", ok=False)
            require(drift.returncode != 0, "request drift accepted")
            h.command("chat", "work")
            result = h.state()
            assert_final(result, 16)
            require(len(result["generations"]) == 2 and len(result["tools"]) == 2, "wrong model/Tool path")
            h.command("chat", "work")
            require(h.state() == result, "draining terminal work changed state")
            return result

        report["concurrent"] = scenario("concurrent", concurrent)
        report["checks"].append("16 simultaneous CLI inputs; one Main; ordered history; idempotency and drift rejection")

        for boundary in ("before_generation_dispatch", "before_generation_commit", "after_generation_commit",
                         "before_conversation_append", "after_conversation_append", "before_reply_checkpoint",
                         "after_reply_checkpoint", "before_tool_commit", "after_tool_commit"):
            def crash(h):
                h.setup()
                h.send("one", "recover me")
                child = h.hold(boundary)
                child.kill()
                child.communicate(timeout=10)
                if boundary == "after_generation_commit":
                    h.send("late", "late crash steering")
                # Read state directly before recovery; history readback may
                # itself fill an immutable Conversation intent, by design.
                before = h.state()
                accepted = {g["generation_id"]: g for g in before["generations"] if g["status"] == "completed"}
                accepted_tools = {t["invocation_id"]: t for t in before["tools"] if t["status"] == "succeeded"}
                h.command("chat", "work")
                after = h.state()
                steered = boundary == "after_generation_commit"
                assert_final(after, 2 if steered else 1)
                require(len(after["generations"]) == (3 if steered else 2) and len(after["tools"]) == (4 if steered else 2), "crash repeated a logical generation/Tool")
                for g in after["generations"]:
                    if g["generation_id"] in accepted:
                        require(g == accepted[g["generation_id"]], "completed generation replayed or drifted")
                for t in after["tools"]:
                    if t["invocation_id"] in accepted_tools:
                        require(t == accepted_tools[t["invocation_id"]], "completed Tool replayed")
                return {"boundary": boundary, "signal": "SIGKILL", "before": before, "after": after}
            report["crashes"].append(scenario(boundary, crash))

        def steering(h):
            old_profile = h.setup()
            first = h.send("first", "original")
            child = h.hold("after_generation_commit")
            for key, text in (("second", "steer one"), ("third", "steer two")):
                require(h.send(key, text)["run_id"] == first["run_id"], "steering spawned Main")
            new_profile = h.setup("second-model")
            child.send_signal(signal.SIGTERM)
            child.communicate(timeout=15)
            with sqlite3.connect(Path(h.env["ERII_DATA_ROOT"]) / "state.db") as db:
                require(db.execute("SELECT status FROM run").fetchone()[0] == "running", "shutdown cancelled/lost business work")
            h.command("chat", "work")
            state = h.state()
            assert_final(state, 3)
            require(len(state["generations"]) == 3, "steering failed to overtake stale reply")
            requests = [json.loads(g["request_json"]) for g in state["generations"]]
            require(all(r["model_snapshot"]["profile_id"] == old_profile for r in requests), "hot model switch")
            require([m["text"] for m in requests[1]["messages"][-2:]] == ["steer one", "steer two"], "steering not separate/ordered")
            h.send("fourth", "new run")
            h.command("chat", "work")
            final = h.state()
            assert_final(final, 4, 2)
            require(json.loads(final["generations"][-1]["request_json"])["model_snapshot"]["profile_id"] == new_profile, "new Run missed profile switch")
            return final
        report["steering"] = scenario("steering", steering)
        report["checks"].append("SIGTERM preserves business work; separate steering at stable boundary; frozen model and new-Run switch")
        report["checks"].append("SIGKILL at nine model/Tool/Conversation/checkpoint boundaries; completed results reused; missing writes filled")
    report["checks"].append("cleanup: all owned processes reaped and disposable databases removed")
    return report


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--evidence", type=Path, required=True)
    args = parser.parse_args()
    args.evidence.unlink(missing_ok=True)
    report = run(args.binary.resolve())
    report["binary_sha256"] = hashlib.sha256(args.binary.read_bytes()).hexdigest()
    args.evidence.parent.mkdir(parents=True, exist_ok=True)
    args.evidence.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"checks": report["checks"], "crash_boundaries": len(report["crashes"]), "evidence": str(args.evidence)}, ensure_ascii=False))


if __name__ == "__main__":
    main()
