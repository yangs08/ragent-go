#!/usr/bin/env python3
"""Exercise the built processes without contacting an external model provider."""
import json
import os
from pathlib import Path
import signal
import select
import socket
import subprocess
import threading
import tempfile
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.request import urlopen, Request
from urllib.error import HTTPError

ROOT = Path(__file__).resolve().parents[1]

def check(workdir):
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    env = {k: v for k, v in os.environ.items() if not k.startswith("RAGENT_")}
    dotenv = workdir / ".env"
    dotenv.write_text(f"RAGENT_HTTP_ADDR=127.0.0.1:{port}\nRAGENT_SHUTDOWN_TIMEOUT=1s\nRAGENT_LOG_LEVEL=INFO\n")
    proc = subprocess.Popen([str(ROOT / "bin/server")], env=env, cwd=workdir,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    try:
        deadline = time.monotonic() + 5
        while True:
            try:
                with urlopen(f"http://127.0.0.1:{port}/healthz", timeout=0.5) as response:
                    assert json.load(response) == {"status": "ok"}
                    request_id = response.headers["X-Request-ID"]
                    assert request_id
                break
            except OSError:
                if proc.poll() is not None or time.monotonic() >= deadline:
                    raise RuntimeError("server failed to become available")
                time.sleep(0.02)
        with urlopen(Request(f"http://127.0.0.1:{port}/healthz", method="HEAD"), timeout=0.5) as response:
            assert response.status == 200 and response.read() == b""
        try:
            urlopen(Request(f"http://127.0.0.1:{port}/healthz", method="POST"), timeout=0.5)
            raise AssertionError("POST /healthz must return 405")
        except HTTPError as response:
            try:
                assert response.code == 405
                assert "GET" in response.headers["Allow"] and "HEAD" in response.headers["Allow"]
            finally:
                response.close()
        proc.send_signal(signal.SIGTERM)
        output, errors = proc.communicate(timeout=3)
        assert proc.returncode == 0, errors
        assert request_id in errors
        assert "server stopped gracefully" in errors
        events = [json.loads(line) for line in errors.splitlines() if line.startswith("{")]
        requests_logged = [event for event in events if event.get("msg") == "http request completed"]
        assert [event["status"] for event in requests_logged] == [200, 200, 405]
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.communicate()
    bad = subprocess.run([str(ROOT / "bin/server")],
                         env=dict(env, RAGENT_HTTP_ADDR="invalid"), cwd=workdir,
                         capture_output=True, text=True, timeout=3)
    assert bad.returncode != 0 and "server stopped" in bad.stderr
    print("PASS: Gin GET/HEAD/405, structured request logs, SIGTERM shutdown, invalid config")

    requests = []
    class Provider(BaseHTTPRequestHandler):
        def do_POST(self):
            body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            requests.append((self.path, self.headers.get("Authorization"), body))
            if not body.get("stream"):
                if body["messages"][-1]["role"] == "tool":
                    message = {"role": "assistant", "content": "A100 已发货"}
                    finish = "stop"
                else:
                    message = {"role": "assistant", "tool_calls": [{"id": "smoke-order-call", "type": "function", "function": {"name": "get_order", "arguments": '{"order_id":"A100"}'}}]}
                    finish = "tool_calls"
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps({"choices": [{"index": 0, "message": message, "finish_reason": finish}]}).encode())
                return
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            if body.get("tools"):
                last = body["messages"][-1]
                if last["role"] == "tool":
                    deltas = [{"role": "assistant", "content": "A100"}, {"content": " 已发货"}]
                elif last["content"] == "它发货了吗？":
                    assert any(m.get("tool_call_id") == "smoke-stream-call" for m in body["messages"])
                    deltas = [{"role": "assistant", "content": "它已发货"}]
                else:
                    deltas = [
                        {"role": "assistant", "tool_calls": [{"index": 0, "id": "smoke-stream-call", "type": "function", "function": {"name": "get_order", "arguments": '{"order_'}}]},
                        {"tool_calls": [{"index": 0, "function": {"arguments": 'id":"A100"}'}}]},
                    ]
            else:
                deltas = [{"role": "assistant", "content": "local-eino-ok"}]
            for delta in deltas:
                chunk = {"choices": [{"index": 0, "delta": delta}]}
                self.wfile.write(("data: " + json.dumps(chunk) + "\n\n").encode())
                self.wfile.flush()
            self.wfile.write(b"data: [DONE]\n\n")
        def log_message(self, *args):
            pass
    provider = ThreadingHTTPServer(("127.0.0.1", 0), Provider)
    thread = threading.Thread(target=provider.serve_forever, daemon=True)
    thread.start()
    try:
        dotenv.write_text(dotenv.read_text() +
                          f"RAGENT_MODEL_BASE_URL=http://127.0.0.1:{provider.server_port}/v1\n"
                          "RAGENT_MODEL_NAME=file-model\nRAGENT_MODEL_API_KEY=local-test-key\n"
                          "RAGENT_MODEL_TIMEOUT=2s\n")
        model_env = dict(env, RAGENT_MODEL_NAME="smoke-model")
        check = subprocess.run([str(ROOT / "bin/modelcheck"), "-prompt", "local smoke test"], env=model_env, cwd=workdir,
                               capture_output=True, text=True, timeout=5)
        assert check.returncode == 0, check.stderr
        assert check.stdout == "local-eino-ok"
        assert len(requests) == 1
        path, auth, body = requests[0]
        assert path == "/v1/chat/completions" and auth == "Bearer local-test-key"
        assert body["stream"] is True and body["model"] == "smoke-model"
        assert body["messages"][0]["content"] == "local smoke test"
        agent_check = subprocess.run([str(ROOT / "bin/agentcheck")], env=model_env, cwd=workdir,
                                     capture_output=True, text=True, timeout=5)
        assert agent_check.returncode == 0, agent_check.stderr
        events = [json.loads(line) for line in agent_check.stdout.splitlines()]
        assert [event["type"] for event in events] == ["tool_call", "tool_result", "answer"]
        assert events[0]["tool_call_id"] == events[1]["tool_call_id"] == "smoke-order-call"
        assert events[1]["result"]["code"] == "ok"
        assert events[1]["result"]["order"]["status"] == "shipped"
        assert events[2]["content"] == "A100 已发货"
        assert len(requests) == 3
        second_model_input = requests[2][2]["messages"][-1]
        assert second_model_input["role"] == "tool" and second_model_input["tool_call_id"] == "smoke-order-call"
        assert json.loads(second_model_input["content"])["code"] == "ok"
        print("PASS: agentcheck process -> ADK -> get_order -> correlated tool result -> answer")
        interactive = subprocess.run([str(ROOT / "bin/agentcheck"), "-interactive"],
                                     input="它发货了吗？\n/exit\n", env=model_env, cwd=workdir,
                                     capture_output=True, text=True, timeout=5)
        assert interactive.returncode == 0, interactive.stderr
        stream_events = [json.loads(line) for line in interactive.stdout.splitlines()]
        assert [e["type"] for e in stream_events] == ["tool_call", "tool_result", "text_delta", "text_delta", "answer", "text_delta", "answer"]
        assert stream_events[2]["message_id"] == stream_events[3]["message_id"] == stream_events[4]["message_id"]
        assert stream_events[-1]["content"] == "它已发货"
        assert len(requests) == 6 and len(requests[-1][2]["messages"]) == 6
        print("PASS: interactive agentcheck -> fragmented tool arguments -> text deltas -> full follow-up history")

        # stdin 保持打开且没有下一行时，SIGINT 也必须能退出读取等待。
        idle = subprocess.Popen([str(ROOT / "bin/agentcheck"), "-interactive"], env=model_env, cwd=workdir,
                                stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            captured = b""
            deadline = time.monotonic() + 5
            while b'"type":"answer"' not in captured:
                remaining = deadline - time.monotonic()
                assert remaining > 0, "interactive process did not finish its first turn"
                readable, _, _ = select.select([idle.stdout], [], [], remaining)
                assert readable, "interactive output timed out"
                data = os.read(idle.stdout.fileno(), 4096)
                assert data, "interactive process ended unexpectedly"
                captured += data
            idle.send_signal(signal.SIGINT)
            idle.wait(timeout=3)
            assert idle.returncode != 0, "cancelled process unexpectedly reported success"
            print("PASS: SIGINT unblocks interactive stdin waiting")
        finally:
            if idle.poll() is None:
                idle.kill()
            idle.communicate(timeout=3)
    finally:
        provider.shutdown()
        provider.server_close()
        thread.join(timeout=2)
    print("PASS: modelcheck process -> .env + system override -> Eino -> local provider")
    dotenv.write_text("BROKEN='private-fixture-value")
    broken = subprocess.run([str(ROOT / "bin/server")], env=env, cwd=workdir,
                            capture_output=True, text=True, timeout=3)
    assert broken.returncode != 0 and "cannot load .env" in broken.stderr
    assert "private-fixture-value" not in broken.stderr
    print("PASS: malformed .env fails safely without leaking file contents")

if __name__ == "__main__":
    with tempfile.TemporaryDirectory(prefix="ragent-go-smoke-") as directory:
        check(Path(directory))
