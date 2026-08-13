"""Integration fixtures: build Core, run it against a dedicated test database
behind the devproxy, and expose its API and the bootstrap code."""

from __future__ import annotations

import base64
import os
import re
import shutil
import subprocess
import time
import urllib.request
from pathlib import Path

import pytest
from devproxy import DevProxy, http_json

REPO = Path(__file__).resolve().parents[2]
CORE_BIN = REPO / "core" / "autosecrets-core"
PG_PORT = os.environ.get("AUTOSECRETS_TEST_PG_PORT", "55433")
PG_USER = "autosecrets"
PG_PASSWORD = "test"
TEST_DB = "autosecrets_agent"


def _wait_health(url: str, timeout: float = 30) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            with urllib.request.urlopen(url + "/api/v1/health", timeout=2) as resp:
                if resp.status == 200:
                    return
        except Exception:
            time.sleep(0.3)
    raise TimeoutError(f"core did not become healthy at {url}")


def _pg_available() -> bool:
    import subprocess
    try:
        result = subprocess.run(
            ["docker", "exec", "autosecrets-test-pg", "pg_isready", "-U", "autosecrets"],
            capture_output=True, timeout=10)
        return result.returncode == 0
    except Exception:
        return False


@pytest.fixture(scope="session")
def core_stack(tmp_path_factory):
    if not _pg_available():
        pytest.skip("test postgres container 'autosecrets-test-pg' is not running; "
                    "start it with: docker run -d --name autosecrets-test-pg "
                    "-e POSTGRES_DB=autosecrets -e POSTGRES_USER=autosecrets "
                    "-e POSTGRES_PASSWORD=test -p 55433:5432 postgres:17-alpine")
    """Builds and starts Core + devproxy once per test session."""
    go_bin = shutil.which("go") or next((p for p in
        ("/home/ichiyo/sdk/go1.26.5/bin/go", "/usr/local/go/bin/go") if os.path.exists(p)), None)
    if go_bin is None:
        raise RuntimeError("go toolchain not found (set PATH or AUTOSECRETS_GO)")
    build_env = {**os.environ, "PATH": os.path.dirname(go_bin) + os.pathsep + os.environ.get("PATH", ""),
                 "GOPROXY": os.environ.get("GOPROXY", "https://proxy.golang.org,direct"),
                 "GOSUMDB": os.environ.get("GOSUMDB", "off")}
    subprocess.run([go_bin, "build", "-o", str(CORE_BIN), "./cmd/autosecrets-core"],
                   cwd=REPO / "core", check=True, env=build_env)
    work = tmp_path_factory.mktemp("core")
    keys = work / "keys"
    keys.mkdir()
    artifact = work / "artifacts"
    artifact.mkdir()

    # Dedicated test database on the shared test PostgreSQL server.
    subprocess.run(["docker", "exec", "autosecrets-test-pg", "psql", "-U", PG_USER, "-d", "postgres",
                    "-c", f"DROP DATABASE IF EXISTS {TEST_DB} WITH (FORCE)"], check=True,
                   capture_output=True)
    subprocess.run(["docker", "exec", "autosecrets-test-pg", "psql", "-U", PG_USER,
                    "-c", f"CREATE DATABASE {TEST_DB}"], check=True, capture_output=True)

    env = {
        **os.environ,
        "CORE_LISTEN_ADDR": "127.0.0.1:18080",
        "CORE_KEYS_DIR": str(keys),
        "CORE_DB_DSN": f"postgres://{PG_USER}:{PG_PASSWORD}@localhost:{PG_PORT}/{TEST_DB}",
        "CORE_TRUSTED_PROXY_CIDRS": "127.0.0.0/8",
        "CORE_PUBLIC_AGENT_URL": "https://agent.test",
        "CORE_ARTIFACT_DIR": str(artifact),
        "GOPROXY": os.environ.get("GOPROXY", "https://goproxy.io,direct"),
        "GOSUMDB": os.environ.get("GOSUMDB", "off"),
    }
    proc = subprocess.Popen([str(CORE_BIN)], env=env,
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    core_url = "http://127.0.0.1:18080"
    log_lines: list[str] = []
    import threading

    def _drain() -> None:
        assert proc.stdout is not None
        for line in proc.stdout:
            log_lines.append(line)

    threading.Thread(target=_drain, daemon=True).start()
    proxy = None
    try:
        _wait_health(core_url)
        deadline = time.time() + 15
        code_match = None
        while time.time() < deadline and code_match is None:
            for line in log_lines:
                code_match = re.search(r"BOOTSTRAP CODE: (\S+)", line)
                if code_match:
                    break
            if code_match is None:
                time.sleep(0.2)
        proxy = DevProxy("127.0.0.1:18080", keys / "agent-ca.crt", keys / "agent-ca.key")
        proxy.start()
        # Raw Ed25519 public key (32 bytes) in base64, as the envelope
        # protocol defines; SPKI DER would be rejected by the agent.
        der = subprocess.run(
            ["openssl", "pkey", "-in", str(keys / "core-signing.key"), "-pubout",
             "-outform", "DER"], check=True, capture_output=True).stdout
        raw = der[-32:]
        yield {
            "core_url": core_url,
            "proxy_url": proxy.url,
            "keys": keys,
            "artifact_dir": artifact,
            "bootstrap_code": code_match.group(1) if code_match else None,
            "signing_public": base64.b64encode(raw).decode(),
            "core_process": proc,
            "log_lines": log_lines,
        }
    finally:
        if proxy is not None:
            proxy.stop()
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()



@pytest.fixture(scope="session")
def admin(core_stack):
    """Bootstraps the password-only Administrator and returns its session."""
    code = core_stack["bootstrap_code"]
    if not code:
        raise AssertionError("no bootstrap code captured from Core logs")
    status, body, headers = http_json("POST", core_stack["core_url"] + "/api/v1/bootstrap", {
        "code": code, "organization_name": "Agent Test Organization",
        "username": "admin", "password": "correct-horse-42"})
    assert status == 201, body
    set_cookie = headers.get("Set-Cookie", "")
    cookie_match = re.search(r"autosecrets_session=([^;]+)", set_cookie)
    assert cookie_match is not None, headers
    cookie = cookie_match.group(1)
    csrf = body["csrf_token"]
    return {"cookie": f"autosecrets_session={cookie}", "csrf": csrf,
            "core_url": core_stack["core_url"]}

def api(
    admin_session,
    method: str,
    path: str,
    body: dict | None = None,
) -> tuple[int, dict, dict[str, str]]:
    return http_json(method, admin_session["core_url"] + path, body,
                     cookies=admin_session["cookie"],
                     headers={"X-CSRF-Token": admin_session["csrf"]})
