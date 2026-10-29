"""Local PostgreSQL auth integration. Never prints credentials or tokens.
Creates uniquely named test users and deletes only those users on completion.
Requires the project's dedicated Compose PostgreSQL and applied migrations.
"""
import base64
import json
import os
from pathlib import Path
import secrets
import signal
import socket
import subprocess
import time
import urllib.request

ROOT = Path(__file__).resolve().parents[1]

def db(sql, env):
    command = ["docker", "compose", "--env-file", ".env", "-f", "deploy/compose.yaml",
               "exec", "-T", "postgres", "sh", "-c",
               'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "$1"', "sh", sql]
    result = subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True, timeout=20)
    if result.returncode:
        raise RuntimeError("project database operation failed")
    return result.stdout.strip()

def main():
    # Derive an isolated check environment from this project's container;
    # never replace the user's .env or accidentally test another database.
    inspected = subprocess.run(["docker", "inspect", "ragent-go-study-postgres-1"],
                               text=True, capture_output=True, timeout=10)
    if inspected.returncode:
        raise RuntimeError("dedicated PostgreSQL container missing; run make db-up first")
    container = json.loads(inspected.stdout)[0]
    values = dict(item.split("=", 1) for item in container["Config"]["Env"] if "=" in item)
    bindings = container["NetworkSettings"]["Ports"]["5432/tcp"]
    binding = next(item for item in bindings if item["HostIp"] == "127.0.0.1")
    check_env = os.environ.copy()
    check_env.update(RAGENT_DB_ENABLED="true", RAGENT_DB_HOST="127.0.0.1",
                     RAGENT_DB_PORT=binding["HostPort"], RAGENT_DB_USER=values["POSTGRES_USER"],
                     RAGENT_DB_NAME=values["POSTGRES_DB"], RAGENT_DB_PASSWORD=values["POSTGRES_PASSWORD"],
                     RAGENT_DB_SSLMODE="disable")

    def check_db(sql):
        return db(sql, check_env)

    suffix = secrets.token_hex(6)
    password = "AuthSmoke_" + secrets.token_urlsafe(24)
    ids = []
    names = ["auth_smoke_a_" + suffix, "auth_smoke_b_" + suffix]
    tokens = []
    logs = []
    process = None
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    base = f"http://127.0.0.1:{port}"

    def start():
        nonlocal process
        env = check_env.copy()
        env.update(RAGENT_HTTP_ADDR=f"127.0.0.1:{port}",
                   RAGENT_AUTH_LOGIN_ATTEMPTS="6", RAGENT_AUTH_LOGIN_WINDOW="1h")
        process = subprocess.Popen([str(ROOT / "bin/server")], cwd=ROOT, env=env,
                                   stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        for _ in range(100):
            try:
                with urllib.request.urlopen(base + "/healthz", timeout=.5) as response:
                    assert json.load(response)["status"] == "ok"
                return
            except OSError:
                if process.poll() is not None:
                    raise RuntimeError("auth server startup failed")
                time.sleep(.1)
        raise RuntimeError("auth server startup timeout")

    def stop():
        nonlocal process
        if process is not None:
            process.send_signal(signal.SIGTERM)
            out, _ = process.communicate(timeout=10)
            logs.append(out)
            assert process.returncode == 0, "auth server shutdown failed"
            process = None

    def call(method, path, payload=None, token=None, raw=None):
        data = raw if raw is not None else (json.dumps(payload).encode() if payload is not None else None)
        headers = {"Content-Type": "application/json"}
        if token:
            headers["Authorization"] = token
        request = urllib.request.Request(base + path, data=data, method=method, headers=headers)
        with urllib.request.urlopen(request, timeout=10) as response:
            assert response.status == 200
            assert response.headers.get("Cache-Control") == "no-store"
            body = json.load(response)
            assert body.get("requestId") == response.headers.get("X-Request-ID")
            return body

    try:
        for name in names:
            env = check_env.copy()
            env["RAGENT_USER_PASSWORD"] = password
            result = subprocess.run([str(ROOT / "bin/usercreate"), "-username", name], cwd=ROOT,
                                    env=env, text=True, capture_output=True, timeout=20)
            assert result.returncode == 0, "create test user failed"
            record = json.loads(result.stdout)
            assert record["userId"].isalnum(), "unexpected generated identifier"
            ids.append(record["userId"])
            assert password not in result.stdout + result.stderr
        start()
        for name, user_id in zip(names, ids):
            reply = call("POST", "/auth/login", {"username": name, "password": password})
            assert reply["code"] == "0" and reply["data"]["userId"] == user_id
            token = reply["data"]["token"]
            assert len(base64.urlsafe_b64decode(token + "=")) == 32
            tokens.append(token)
        missing = call("POST", "/auth/login", {"username": "missing_" + suffix, "password": password})
        wrong = call("POST", "/auth/login", {"username": names[0], "password": "wrong-password"})
        assert missing["code"] == wrong["code"] == "A000001"
        assert missing["message"] == wrong["message"]
        assert call("POST", "/auth/login", {"username": names[0], "password": password, "userId": ids[1]})["code"] != "0"
        assert call("POST", "/auth/login", raw=b'{}{}')["code"] != "0"
        limited = call("POST", "/auth/login", {"username": names[0], "password": password})
        assert limited["code"] != "0" and "频繁" in limited["message"]
        for token, user_id in zip(tokens, ids):
            me = call("GET", "/user/me", token=token)
            assert me["code"] == "0" and me["data"]["userId"] == user_id
            assert "password" not in json.dumps(me).lower()
        forged = secrets.token_urlsafe(32)
        assert call("GET", "/user/me", token=forged)["code"] != "0"
        assert call("GET", "/user/me")["code"] != "0"
        for user_id in ids:
            stored = check_db(f"SELECT password_hash FROM users WHERE id = '{user_id}'")
            assert stored.startswith("$2") and password not in stored
        assert check_db(f"SELECT length(token_hash) FROM auth_sessions WHERE user_id = '{ids[1]}'") == "64"
        assert call("POST", "/auth/logout", token=tokens[0])["code"] == "0"
        assert call("GET", "/user/me", token=tokens[0])["code"] != "0"
        assert call("GET", "/user/me", token=tokens[1])["code"] == "0"
        stop()
        start()
        assert call("GET", "/user/me", token=tokens[1])["code"] == "0"
        check_db(f"UPDATE auth_sessions SET expires_at = CURRENT_TIMESTAMP - INTERVAL '1 minute' WHERE user_id = '{ids[1]}'")
        assert call("GET", "/user/me", token=tokens[1])["code"] != "0"
        stop()
        for private in [password, *tokens]:
            assert private not in "".join(logs), "credential leaked to application logs"
        print("PASS: login, credential errors, strict input, rate limit, per-user context, forged/expired/revoked tokens")
        print("PASS: password hashing, token digest storage, session survives restart, secrets absent from logs")
    finally:
        if process is not None:
            if process.poll() is None:
                process.kill()
            out, _ = process.communicate(timeout=5)
            logs.append(out)
        for user_id in ids:
            check_db(f"DELETE FROM users WHERE id = '{user_id}'")
        print("Test users and their sessions cleaned up")

if __name__ == "__main__":
    main()
