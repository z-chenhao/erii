#!/usr/bin/env python3
"""Owned HTTP endpoint -> actual Go CLI -> actual pinned Node/Pi Bridge.

Launch/doctor/drive/evidence/cleanup are one foreground, disposable operation.
"""
import argparse
from contextlib import closing
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import shutil
import sqlite3
import subprocess
import tempfile
import threading
import time


def require(ok, message):
    if not ok:
        raise RuntimeError(message)


class Endpoint(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *args):
        pass

    def do_GET(self):
        self.server.requests.append({"method": "GET", "auth": self.headers.get("Authorization")})
        if self.server.mode == "manual":
            self.send_response(404)
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        data = json.dumps({"data": [{"id": "fixture-model"}]}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        self.server.requests.append({"method": "POST", "auth": self.headers.get("Authorization"),
                                     "session": self.headers.get("session_id"), "body": body})
        if self.server.mode == "auth":
            self.send_response(401)
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Connection", "close")
        self.end_headers()

        def chunk(delta, finish=None, usage=None):
            value = {"id": "fixture", "object": "chat.completion.chunk", "model": body["model"],
                     "choices": [{"index": 0, "delta": delta, "finish_reason": finish}]}
            if usage:
                value["usage"] = usage
            self.wfile.write(("data: " + json.dumps(value) + "\n\n").encode())
            self.wfile.flush()

        try:
            text = str(body["messages"][-1].get("content", ""))
            if "until cancelled" in text:
                for _ in range(100):
                    chunk({"content": "streaming "})
                    time.sleep(.01)
            elif self.server.mode == "tasks":
                names = [item["function"]["name"] for item in body.get("tools", [])]
                root = any("general Worker executing" in str(m.get("content", "")) for m in body["messages"])
                runtime = any("Runtime task update (untrusted Worker data" in str(m.get("content", "")) for m in body["messages"])
                has_result = any(m["role"] == "tool" for m in body["messages"])
                if runtime and not root:
                    chunk({"content": "Main Task follow-up through Pi."})
                elif not has_result and (root or "builtin__task__manage" in names):
                    name = "builtin__text__echo" if root else "builtin__task__manage"
                    arguments = {"text": "Independent Root report."} if root else {"operation": "create", "task": "Prepare a safe independent report. Return evidence; perform no external actions."}
                    chunk({"tool_calls": [{"index": 0, "id": "task_call", "type": "function", "function": {"name": name, "arguments": json.dumps(arguments)}}]})
                else:
                    chunk({"content": "Root business result through Pi." if root else "Main Task accepted through Pi."})
                chunk({}, "stop", {"prompt_tokens": 7, "completion_tokens": 8, "total_tokens": 15})
            elif self.server.mode == "partial":
                chunk({"content": "partial"})
                return
            elif body.get("tools") and not any(m["role"] == "tool" for m in body["messages"]):
                if self.server.mode == "no-tools":
                    chunk({"content": "I cannot call tools"})
                    chunk({}, "stop")
                else:
                    names = [item["function"]["name"] for item in body["tools"]]
                    name = "builtin__text__echo" if "builtin__text__echo" in names else names[0]
                    arguments = {"text": text} if name == "builtin__text__echo" else {"value": "OK"}
                    chunk({"tool_calls": [{"index": 0, "id": "call_echo", "type": "function",
                                          "function": {"name": name, "arguments": json.dumps(arguments)}}]})
                    chunk({}, "tool_calls")
            else:
                chunk({"content": "PROBE_RESULT_OK" if any(m["role"] == "tool" for m in body["messages"])
                       else "ABCDEFGHIJKLMNOPQRSTUVWXYZ" if "long output" in text else "OK"})
                chunk({}, "stop", {"prompt_tokens": 7, "completion_tokens": 8, "total_tokens": 15})
            self.wfile.write(b"data: [DONE]\n\n")
        except (BrokenPipeError, ConnectionResetError):
            pass
        finally:
            self.close_connection = True


def verify(binary, script):
    require(shutil.which("node") is not None, "Node is missing")
    require((script.parent / "node_modules/@earendil-works/pi-ai").is_dir(), "run npm ci --prefix provider-bridge")
    server = ThreadingHTTPServer(("127.0.0.1", 0), Endpoint)
    server.daemon_threads = True
    server.requests, server.mode = [], "success"
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    report = {"bridge": "pi-ai-1.3.0-v1", "checks": []}
    try:
        with tempfile.TemporaryDirectory(prefix="erii-local-") as directory:
            root = Path(directory)
            endpoint = f"http://127.0.0.1:{server.server_port}/v1"
            env = {k: v for k, v in os.environ.items() if not k.startswith("ERII_")}
            env["ERII_DATA_ROOT"] = str(root / "data")
            env["ERII_FIXTURE_KEY"] = "SYNTHETIC_LOCAL_KEY_NEVER_PERSIST"
            config = root / "config.yaml"
            config.write_text("version: 1\nllm:\n  provider_bridge_command: node\n  provider_bridge_script: " + json.dumps(str(script)) + "\n  provider_bridge_version: pi-ai-1.3.0-v1\n")

            def command(*args, ok=True, text=None):
                p = subprocess.run([str(binary), "-config", str(config), *args], cwd=root, env=env,
                                   input=text, text=True, capture_output=True, timeout=20)
                require((p.returncode == 0) == ok, f"CLI {args[0:2]} unexpected result: {p.stderr}")
                require("SYNTHETIC_LOCAL_KEY_NEVER_PERSIST" not in p.stdout + p.stderr, "credential in CLI output")
                return json.loads(p.stdout) if ok else p.stderr

            require(command("status")["readiness"] == "setup_required", "implicit model")
            require(command("model", "discover", endpoint)[0]["model_id"] == "fixture-model", "catalog")
            profile = command("model", "add", endpoint, "fixture-model")
            id_ = profile["profile_id"]
            command("model", "select", id_, "1", "1", ok=False)
            require(command("status")["readiness"] == "setup_required", "unverified activation")
            proof = command("model", "test", id_)
            require(all(proof["result"].values()), "probe did not exercise every behavior")
            require(command("model", "select", id_, "1", "1")["readiness"] == "ready", "select")
            require(command("status")["readiness"] == "ready", "restart readiness")
            command("chat", "send", "local-main", text="actual local conversation")
            command("chat", "work")
            history = command("chat", "history")
            require([m["role"] for m in history] == ["user", "assistant"], "local Main history")
            require(history[-1]["content"][0]["text"] == "PROBE_RESULT_OK", "local Main did not continue after Direct Tool")
            require(any(m["role"] == "system" for r in server.requests if r.get("body") for m in r["body"].get("messages", [])), "Main System Prompt missing from actual wire")
            report["main_history"] = history
            report["checks"].append("actual CLI Main Actor/Loop/Direct Tool/Conversation through local Node/Pi with System Prompt")
            require(all(r["auth"] is None for r in server.requests), "fabricated Authorization")
            report["checks"] += ["catalog", "manual CLI add", "unverified rejection", "real five-part probe", "atomic select", "restart ready", "no fabricated key"]

            server.mode = "tasks"
            task_start = len(server.requests)
            command("chat", "send", "local-task", text="Delegate an independent report in the background.")
            command("chat", "work")
            task_history = command("chat", "history")
            require(task_history[-1]["content"][0]["text"] == "Main Task follow-up through Pi.", "actual Pi Task follow-up missing")
            task_requests = [r["body"] for r in server.requests[task_start:] if r.get("body")]
            roots = [r for r in task_requests if any("general Worker executing" in str(m.get("content", "")) for m in r["messages"])]
            require(roots and all(not any(t["function"]["name"].startswith("builtin__task__") for t in r["tools"]) for r in roots), "Root Task control exposed on actual Pi wire")
            require(all("actual local conversation" not in json.dumps(r["messages"]) for r in roots), "Root inherited Main Conversation")
            require(any(any(m["role"] == "user" and "Runtime task update (untrusted Worker data" in str(m.get("content", "")) for m in r["messages"]) for r in task_requests), "runtime framing lost at actual Pi boundary")
            with closing(sqlite3.connect(root / "data/state.db")) as db:
                require(db.execute("SELECT count(*) FROM task WHERE status='completed'").fetchone()[0] == 1, "actual Pi Task missing")
                require(db.execute("SELECT count(*) FROM main_inbox").fetchone()[0] == 1, "actual Pi Reporter dedup missing")
            report["task_requests"] = task_requests
            report["task_history"] = task_history
            report["checks"].append("actual Node/Pi Task create -> Root echo/result -> runtime-framed Reporter -> one Main follow-up; role/context isolation")

            # A failed retest must revoke proof and close ingress after endpoint failure.
            server.mode = "no-tools"
            require("MODEL_INCOMPATIBLE" in command("model", "test", id_, ok=False), "missing tools accepted")
            require(command("status")["readiness"] == "degraded", "failed retest left ingress open")
            report["checks"].append("failed retest revokes active proof")

            server.mode = "manual"
            require("MODEL_NOT_FOUND" in command("model", "discover", endpoint, ok=False), "missing catalog")
            manual = command("model", "add", endpoint, "manual-id")["profile_id"]
            require(all(command("model", "test", manual)["result"].values()), "manual model probe")
            report["checks"].append("manual ID independent of catalog")

            server.mode = "success"
            auth = command("model", "login", "ERII_FIXTURE_KEY")
            keyed = command("model", "add", endpoint, "fixture-model", auth["connection_id"])["profile_id"]
            start = len(server.requests)
            command("model", "catalog", keyed)
            command("model", "test", keyed)
            require(all(r["auth"] == "Bearer " + env["ERII_FIXTURE_KEY"] for r in server.requests[start:]), "key missing/wrong")
            del env["ERII_FIXTURE_KEY"]
            require("AUTH_REQUIRED" in command("model", "test", keyed, ok=False), "missing key not rejected")
            report["checks"] += ["explicit environment credential", "missing credential rejection"]

            server.mode = "partial"
            require("INVALID_PROVIDER_RESPONSE" in command("model", "test", manual, ok=False), "partial stream completion")
            report["checks"].append("partial stream is not completion")

            with closing(sqlite3.connect(root / "data/state.db")) as db:
                require(db.execute("pragma user_version").fetchone()[0] == 17, "schema")
                rows = db.execute("SELECT snapshot_id,run_id,input_tokens,output_tokens,latency_ms,error_code,snapshot_json FROM provider_usage").fetchall()
                require(rows and any(r[2:4] == (7, 8) for r in rows), "usage not recorded")
                require(any(r[5] == "GENERATION_CANCELLED" and r[2] is None for r in rows), "unknown cancellation usage fabricated")
                require(any(r[5] == "INVALID_PROVIDER_RESPONSE" for r in rows), "failed usage missing")
                require(all(r[4] >= 0 and json.loads(r[6])["snapshot_id"] == r[0] for r in rows), "snapshot attribution")
                require("SYNTHETIC_LOCAL_KEY_NEVER_PERSIST" not in "\n".join(db.iterdump()), "credential in SQLite")
                report["usage"] = [{"snapshot_id": r[0], "run_id": r[1], "input_tokens": r[2], "output_tokens": r[3], "latency_ms": r[4], "error_code": r[5]} for r in rows]
            for file in (root / "data/logs").glob("*"):
                require(b"SYNTHETIC_LOCAL_KEY_NEVER_PERSIST" not in file.read_bytes(), "credential in log")
            report["checks"] += ["SQLite usage and failure readback", "no credentials in DB or logs"]
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=3)
    report["cleanup"] = "all CLI children reaped; endpoint stopped; disposable state removed"
    return report


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", default="bin/personal-agent")
    parser.add_argument("--evidence", default="evidence/local-provider.json")
    args = parser.parse_args()
    result = verify(Path(args.binary).resolve(), Path("provider-bridge/bridge.mjs").resolve())
    Path(args.evidence).parent.mkdir(parents=True, exist_ok=True)
    Path(args.evidence).write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps({"checks": len(result["checks"]), "usage_rows": len(result["usage"]), "cleanup": result["cleanup"]}))
