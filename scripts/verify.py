#!/usr/bin/env python3
"""Launch, doctor, drive, capture, and clean up the real foundation CLI.

Uses only Python's standard library. No user data, accounts, ports, or services.
"""

import argparse
from contextlib import closing
import json
import os
from pathlib import Path
import selectors
import signal
import sqlite3
import subprocess
import tempfile


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def snapshot(data_root):
    with closing(sqlite3.connect((data_root / "state.db").as_uri() + "?mode=ro", uri=True)) as db:
        return {
            "schema_version": db.execute("PRAGMA user_version").fetchone()[0],
            "journal_mode": db.execute("PRAGMA journal_mode").fetchone()[0],
            "rows": db.execute("SELECT * FROM agent_config").fetchall(),
            "dump": list(db.iterdump()),
        }


def diagnostics(value):
    require(value["readiness"] == "setup_required", "incorrect readiness")
    require(value["agent_ingress_enabled"] is False, "ingress enabled without a model")
    require(value["active_model_profile_id"] is None, "implicit model enabled")
    require(value["schema_version"] == 17, "incorrect schema version")


class Harness:
    def __init__(self, binary, root):
        self.binary = binary
        self.root = root
        self.data_root = root / "data with # and ?"
        self.env = {k: v for k, v in os.environ.items() if not k.startswith("ERII_")}
        self.env["ERII_DATA_ROOT"] = str(self.data_root)
        self.config = root / "config.yaml"
        self.yaml_root = root / "unused-config-root"
        self.config.write_text(f"version: 1\napp:\n  data_root: {json.dumps(str(self.yaml_root))}\n  log_level: info\n", encoding="utf-8")
        self.process = None

    def doctor(self):
        require(self.binary.is_file() and os.access(self.binary, os.X_OK), "build binary first")
        result = self.command("-help")
        require(result.returncode == 0 and "Usage:" in result.stderr, "CLI help unavailable")
        require(not self.data_root.exists(), "doctor unexpectedly created application state")

    def command(self, *args, input=None):
        return subprocess.run([str(self.binary), "-config", str(self.config), *args], cwd=self.root, env=self.env,
                              input=input, text=True, capture_output=True, timeout=15, check=False)

    def launch(self, *args, readiness="setup_required"):
        self.process = subprocess.Popen([str(self.binary), "-config", str(self.config), *args], cwd=self.root, env=self.env,
                                        stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        with selectors.DefaultSelector() as selector:
            selector.register(self.process.stdout, selectors.EVENT_READ)
            require(selector.select(timeout=15), "startup diagnostics timed out")
        line = self.process.stdout.readline()
        require(bool(line), "process exited before startup diagnostics")
        value = json.loads(line)
        if readiness == "setup_required":
            diagnostics(value)
        else:
            require(value["readiness"] == readiness, "incorrect launch readiness")
            require(value["agent_ingress_enabled"] == (readiness == "ready"), "incorrect ingress gate")
        require(self.process.poll() is None, "default command did not remain running")
        return value

    def stop(self, sig=signal.SIGTERM):
        self.process.send_signal(sig)
        out, err = self.process.communicate(timeout=15)
        code = self.process.returncode
        self.process = None
        require(not out and not err, "unexpected shutdown output")
        return code

    def cleanup(self):
        if self.process is not None:
            if self.process.poll() is None:
                self.process.kill()
            self.process.communicate(timeout=15)
            self.process = None


def verify_models(harness):
    """Use the metadata CLI, then read its authoritative SQLite state."""
    empty = harness.command("model", "list")
    require(empty.returncode == 0 and json.loads(empty.stdout) == [], "implicit profile created")
    fixture = {
        "name": "Local candidate", "provider_kind": "openai_compatible",
        "endpoint": "http://127.0.0.1:8000/v1", "wire_api": "openai_completions",
        "model_id": "fixture-model", "context_window": 8192, "max_output_tokens": 1024,
        "generation_defaults": {"top_p": 1, "temperature": 0},
        "compatibility": {"tool_result_format": "openai"},
        "capabilities": {"text_input": True},
    }
    created = harness.command("model", "add", input=json.dumps(fixture))
    require(created.returncode == 0 and not created.stderr, "model add failed")
    profile = json.loads(created.stdout)
    require(profile["version"] == 1 and profile["status"] == "unverified" and "verified_at" not in profile,
            "metadata gained verification")
    listed = harness.command("model", "list")
    require(listed.returncode == 0 and json.loads(listed.stdout) == [profile], "new process lost profile")
    before = snapshot(harness.data_root)
    with closing(sqlite3.connect(harness.data_root / "state.db")) as db:
        row = db.execute("SELECT profile_id, status, verified_at, version, created_at, updated_at, generation_defaults_json, compatibility_json, capabilities_json FROM model_profile").fetchone()
        require(row[:4] == (profile["profile_id"], "unverified", None, 1), "SQLite disagrees with CLI")
        require(row[4] == row[5] and row[4] > 10**15, "timestamps are not UTC microseconds")
        canonical = list(row[6:])
        for value in canonical:
            require(value == json.dumps(json.loads(value), sort_keys=True, separators=(",", ":")),
                    "stored model JSON is not canonical")
    baseline_status = harness.launch()
    require(harness.stop(signal.SIGKILL) == -signal.SIGKILL, "metadata kill injection failed")
    restarted = harness.command("model", "list")
    require(restarted.returncode == 0 and json.loads(restarted.stdout) == [profile], "kill/restart lost metadata")
    status = harness.command("status")
    require(status.returncode == 0 and json.loads(status.stdout) == baseline_status, "metadata changed readiness")
    require(snapshot(harness.data_root) == before, "restart rewrote committed metadata")

    rejections = []
    candidates = [dict(fixture, api_key="SYNTHETIC_REDACTION_MARKER"),
                  dict(fixture, endpoint="https://user:SYNTHETIC_REDACTION_MARKER@host/v1"),
                  dict(fixture, endpoint="https://host/v1?key=SYNTHETIC_REDACTION_MARKER"),
                  dict(fixture, status="ready"), dict(fixture, verified_at="2026-10-15T00:00:00Z"),
                  dict(fixture, provider_kind="fake"), dict(fixture, auth_connection_id="missing")]
    for candidate in candidates:
        result = harness.command("model", "add", input=json.dumps(candidate))
        require(result.returncode != 0 and not result.stdout, "invalid profile was accepted")
        require("SYNTHETIC_REDACTION_MARKER" not in result.stderr, "secret value echoed")
        rejections.append(result.stderr.strip())
    activate = harness.command("model", "select")
    require(activate.returncode != 0 and not activate.stdout, "activation exposed before Probe")
    require(snapshot(harness.data_root) == before, "rejected metadata changed SQLite")
    for path in harness.data_root.rglob("*"):
        if path.is_file():
            require(b"SYNTHETIC_REDACTION_MARKER" not in path.read_bytes(), "rejected secret persisted")

    # Execution policy must not turn structurally valid diagnostic metadata
    # into an unreadable row that blocks listing every other candidate.
    large = dict(fixture, name="Over-limit diagnostic candidate", context_window=2097152, max_output_tokens=131072)
    added = harness.command("model", "add", input=json.dumps(large))
    require(added.returncode == 0, "over-limit diagnostic candidate rejected")
    large_profile = json.loads(added.stdout)
    listed = harness.command("model", "list")
    require(listed.returncode == 0 and json.loads(listed.stdout) == [profile, large_profile],
            "over-limit metadata prevented diagnostic listing")
    preserved = snapshot(harness.data_root)
    bounds_rejections = []
    for args in [("test", large_profile["profile_id"]), ("select", large_profile["profile_id"], "1", "1")]:
        rejected = harness.command("-test-bridge", "success", "model", *args)
        require(rejected.returncode != 0 and "Host safety bounds" in rejected.stderr and not rejected.stdout,
                "over-limit diagnostic profile reached activation")
        bounds_rejections.append(rejected.stderr.strip())
    require(snapshot(harness.data_root) == preserved, "rejected over-limit activation changed metadata")
    return {"profile": profile, "canonical_json": canonical, "rejections": rejections,
            "over_limit_metadata": {"profile": large_profile, "rejections": bounds_rejections,
                                    "readback": "both profiles listed; bounds enforced only at activation; SQLite unchanged"},
            "restart": "SIGKILL, new model list process, identical SQLite dump",
            "readiness": baseline_status, "result": "passed"}


def verify_provider(binary):
    """Exercise explicit Bridge injection through real CLI processes and SQL."""
    with tempfile.TemporaryDirectory(prefix="erii-provider-") as temp:
        harness = Harness(binary, Path(temp))
        try:
            harness.doctor()
            initial = harness.command("status")
            require(initial.returncode == 0, "initial status failed")
            diagnostics(json.loads(initial.stdout))
            empty = snapshot(harness.data_root)
            cancelled = harness.command("model", "cancel")
            require(cancelled.returncode == 0 and json.loads(cancelled.stdout) == json.loads(initial.stdout),
                    "cancelled setup changed readiness")
            require(snapshot(harness.data_root) == empty, "cancelled setup changed persistent state")
            fixture = {
                "name": "Explicit test candidate", "provider_kind": "openai_compatible",
                "endpoint": "http://127.0.0.1:8000/v1", "wire_api": "openai_completions",
                "model_id": "fixture-model", "context_window": 8192, "max_output_tokens": 1024,
                "compatibility": {"tool_result_format": "openai"}, "capabilities": {"text_input": True},
            }
            added = harness.command("model", "add", input=json.dumps(fixture))
            require(added.returncode == 0, "candidate add failed")
            profile = json.loads(added.stdout)
            profile_id = profile["profile_id"]
            candidate = snapshot(harness.data_root)
            def bridge(*args, fixture="success"):
                return harness.command("-test-bridge", fixture, *args)
            unprobed = bridge("model", "select", profile_id, "1", "1")
            require(unprobed.returncode != 0 and snapshot(harness.data_root) == candidate,
                    "unprobed profile activated")
            implicit = harness.command("model", "test", profile_id)
            require(implicit.returncode != 0 and snapshot(harness.data_root) == candidate,
                    "implicit provider selected")
            catalog = bridge("model", "catalog")
            require(catalog.returncode == 0 and json.loads(catalog.stdout)[0]["model_id"] == "fixture-model",
                    "Bridge catalog failed")
            incompatible = bridge("model", "test", profile_id, fixture="incompatible")
            require(incompatible.returncode != 0 and "MODEL_INCOMPATIBLE" in incompatible.stderr,
                    "incompatible Bridge accepted")
            no_tools = bridge("model", "test", profile_id, fixture="no-tools")
            require(no_tools.returncode != 0 and "MODEL_INCOMPATIBLE" in no_tools.stderr,
                    "tool-less model passed Probe")
            tested = bridge("model", "test", profile_id)
            require(tested.returncode == 0, "behavioral Probe failed: " + tested.stderr)
            probe = json.loads(tested.stdout)
            require(all(probe["result"].values()) and probe["bridge_identity"] == "test-bridge-v1:success",
                    "Probe did not exercise mandatory behavior")
            tested_state = snapshot(harness.data_root)
            stale = bridge("model", "select", profile_id, "1", "999")
            require(stale.returncode != 0 and snapshot(harness.data_root) == tested_state,
                    "stale AgentConfig changed state")
            wrong_bridge = bridge("model", "select", profile_id, "1", "1", fixture="no-tools")
            require(wrong_bridge.returncode != 0 and snapshot(harness.data_root) == tested_state,
                    "Probe provenance was ignored")
            selected = bridge("model", "select", profile_id, "1", "1")
            require(selected.returncode == 0, "atomic activation failed: " + selected.stderr)
            ready = json.loads(selected.stdout)
            require(ready["readiness"] == "ready" and ready["agent_ingress_enabled"] is True
                    and ready["config_version"] == 2 and ready["active_model_profile_id"] == profile_id,
                    "activation did not open readiness gate")
            with closing(sqlite3.connect(harness.data_root / "state.db")) as db:
                row = db.execute("SELECT status,verified_at,version FROM model_profile WHERE profile_id=?", (profile_id,)).fetchone()
                require(row[0] == "ready" and row[1] > 10**15 and row[2] == 2, "profile activation not stored")
                config = db.execute("SELECT active_model_profile_id,setup_status,config_version FROM agent_config").fetchone()
                require(config == (profile_id, "ready", 2), "AgentConfig disagrees with profile")
                events = db.execute("SELECT type FROM event_outbox WHERE type IN ('ai.erii.model_profile.activated.v1','ai.erii.agent.model_selected.v1') ORDER BY type").fetchall()
                require(len(events) == 2, "activation event pair missing")
            committed = snapshot(harness.data_root)
            require(harness.launch("-test-bridge", "success", readiness="ready") == ready, "ready launch changed state")
            require(harness.stop(signal.SIGKILL) == -signal.SIGKILL, "ready process kill failed")
            restarted = bridge("status")
            require(restarted.returncode == 0 and json.loads(restarted.stdout) == ready,
                    "activation not recovered after SIGKILL")
            no_injection = harness.command("status")
            degraded = json.loads(no_injection.stdout)
            require(no_injection.returncode == 0 and degraded["readiness"] == "degraded"
                    and degraded["agent_ingress_enabled"] is False, "saved Fake enabled implicit ingress")
            incompatible_status = bridge("status", fixture="incompatible")
            require(incompatible_status.returncode == 0
                    and json.loads(incompatible_status.stdout)["readiness"] == "degraded"
                    and json.loads(incompatible_status.stdout)["agent_ingress_enabled"] is False,
                    "incompatible Bridge prevented repair diagnostics or opened ingress")
            require(snapshot(harness.data_root) == committed, "restart rewrote committed activation")
            with closing(sqlite3.connect(harness.data_root / "state.db")) as db:
                db.execute("UPDATE model_profile SET status='unavailable',version=version+1 WHERE profile_id=?", (profile_id,))
                db.commit()
            invalid = bridge("status")
            require(invalid.returncode == 0 and json.loads(invalid.stdout)["readiness"] == "degraded"
                    and json.loads(invalid.stdout)["agent_ingress_enabled"] is False,
                    "invalid active profile did not report closed degraded readiness: " + invalid.stderr + invalid.stdout)
            return {"result": "passed", "initial": json.loads(initial.stdout), "cancelled_setup": "unchanged SQLite dump",
                    "probe": probe, "activated": ready, "stored_profile": row, "stored_config": config,
                    "activation_events": events, "restart": "SIGKILL; fresh Bridge handshake; same SQLite dump",
                    "without_test_injection": degraded, "incompatible_bridge": json.loads(incompatible_status.stdout),
                    "invalid_profile": json.loads(invalid.stdout),
                    "rejected": [unprobed.stderr.strip(), implicit.stderr.strip(), incompatible.stderr.strip(),
                                 no_tools.stderr.strip(), stale.stderr.strip(), wrong_bridge.stderr.strip()]}
        finally:
            harness.cleanup()


def verify(binary):
    with tempfile.TemporaryDirectory(prefix="erii-verify-") as temp:
        harness = Harness(binary, Path(temp))
        checks = []
        try:
            harness.doctor()
            checks.append("doctor: binary and help ready; no state created")
            first = harness.launch()
            stored = snapshot(harness.data_root)
            require(not harness.yaml_root.exists(), "environment did not override YAML data root")
            require(stored["schema_version"] == 17 and stored["journal_mode"] == "wal", "SQLite setup incorrect")
            require(len(stored["rows"]) == 1, "bootstrap is not a singleton")
            require(stored["rows"][0][1] == first["agent_id"], "CLI disagrees with stored identity")
            live_status = harness.command("status")
            require(live_status.returncode == 0 and json.loads(live_status.stdout) == first,
                    "status failed while foreground process was running")
            require(harness.stop() == 0, "SIGTERM did not exit cleanly")
            checks.append("launch/drive: YAML and environment override, fresh database, setup_required, closed ingress, live status, clean SIGTERM")

            repeated = harness.command("status")
            require(repeated.returncode == 0 and not repeated.stderr, "status command failed")
            require(json.loads(repeated.stdout) == first, "restart changed diagnostics")
            require(snapshot(harness.data_root) == stored, "restart changed stored data")
            checks.append("drive: status restart preserves all rows, timestamps, schema, and identity")

            require(harness.launch() == first, "launch changed diagnostics")
            require(harness.stop(signal.SIGKILL) == -signal.SIGKILL, "kill injection failed")
            restarted = harness.command("status")
            require(restarted.returncode == 0, "restart after kill failed")
            require(json.loads(restarted.stdout) == first, "kill/restart changed diagnostics")
            require(snapshot(harness.data_root) == stored, "kill/restart changed stored data")
            checks.append("drive: SIGKILL then restart preserves committed bootstrap state")

            logs = [json.loads(line) for line in (harness.data_root / "logs/app.jsonl").read_text().splitlines()]
            traces = [json.loads(line) for line in (harness.data_root / "logs/traces.jsonl").read_text().splitlines()]
            startup = [s for s in traces if s["name"] == "app.startup"]
            require(len(startup) == 5, "missing startup traces")
            for parent in startup:
                require(any(child["parent_span_id"] == parent["span_id"] and child["trace_id"] == parent["trace_id"]
                            for child in traces), "missing database span correlation")
                require(any(log.get("trace_id") == parent["trace_id"] for log in logs), "missing log correlation")
            checks.append("evidence: valid JSON logs and correlated startup/database spans")

            models = verify_models(harness)
            provider = verify_provider(binary)
            checks.append("models: add/list/readback; canonical JSON; SIGKILL/restart; unverified, closed ingress; invalid input and credential redaction")
            # Include the metadata commands' startup spans in failure-span checks.
            traces = [json.loads(line) for line in (harness.data_root / "logs/traces.jsonl").read_text().splitlines()]
            logs = [json.loads(line) for line in (harness.data_root / "logs/app.jsonl").read_text().splitlines()]

            with closing(sqlite3.connect(harness.data_root / "state.db")) as db:
                db.execute("PRAGMA user_version = 99")
            future_before = snapshot(harness.data_root)
            rejected = harness.command("status")
            require(rejected.returncode != 0 and not rejected.stdout, "future schema was accepted")
            require("unsupported database schema version 99" in rejected.stderr, "future schema diagnostic missing")
            require(snapshot(harness.data_root) == future_before, "future schema rejection changed data")
            failure_traces = [json.loads(line) for line in (harness.data_root / "logs/traces.jsonl").read_text().splitlines()]
            new_spans = failure_traces[len(traces):]
            require(len(new_spans) == 2 and all(s["status"] == "error" for s in new_spans),
                    "failed startup did not record error spans")
            traces = failure_traces
            checks.append("drive: future schema fails closed without changing schema or rows; error spans recorded")

            invalid = harness.root / "invalid.yaml"
            invalid.write_text("version: 1\napi_key: SECRET_SENTINEL\n", encoding="utf-8")
            rejected = harness.command("-config", str(invalid), "status")
            require(rejected.returncode != 0 and not rejected.stdout, "invalid YAML was accepted")
            require("SECRET_SENTINEL" not in rejected.stderr, "config value leaked in error")
            checks.append("drive: unknown configuration rejected without echoing values")
            return {"result": "passed", "scope": "M0 foundation, events, model metadata and test Provider Bridge; no M1 claim",
                    "checks": checks, "diagnostics": first, "stored_rows": stored["rows"],
                    "models": models, "provider": provider, "golden_cases": "not_applicable: first product cases are due at M1",
                    "log_records": len(logs), "trace_records": len(traces),
                    "logs": logs, "traces": traces}
        finally:
            harness.cleanup()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--evidence", type=Path, required=True)
    args = parser.parse_args()
    # Never leave a stale passing report after a failed verification.
    args.evidence.parent.mkdir(parents=True, exist_ok=True)
    args.evidence.unlink(missing_ok=True)
    report = verify(args.binary.resolve())
    report["checks"].append("cleanup: exact child processes reaped; isolated state removed")
    args.evidence.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({key: value for key, value in report.items() if key not in ("logs", "traces")}, indent=2))
    print(f"Full log/span evidence: {args.evidence}")


if __name__ == "__main__":
    main()
