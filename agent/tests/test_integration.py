"""End-to-end Agent interop: enroll through the devproxy (Caddy contract),
poll Desired State, and materialize files on a fake Managed Node."""

from __future__ import annotations

import json
import os
import stat
import subprocess
import sys

import pytest
from conftest import api
from devproxy import http_json


@pytest.fixture(scope="module")
def managed_node(tmp_path_factory, core_stack, admin):
    """Drives the management API to create one Secret and its Assignment,
    then enrolls a node and runs one convergence pass."""
    work = tmp_path_factory.mktemp("node")
    identity_dir = work / "identity"
    bundle_dir = work / "bundles"
    config = work / "config.toml"
    config.write_text(f"""
server_url = "{core_stack['proxy_url']}"
identity_dir = "{identity_dir}"
bundle_dir = "{bundle_dir}"
name = "fixture-node"
signing_public_key = "{core_stack['signing_public']}"
ca_bundle = "{core_stack['keys'] / 'agent-ca.crt'}"
""".lstrip(), encoding="utf-8")

    # Authoring path.
    _, app, _ = api(admin, "POST", "/api/v1/applications", {"name": "payments"})
    _, env, _ = api(admin, "POST", f"/api/v1/applications/{app['id']}/environments",
                    {"name": "production"})
    secret_value = "fixture-db-password-1"
    _, secret, _ = api(admin, "POST",
                       f"/api/v1/applications/{app['id']}/environments/{env['id']}/secrets",
                       {"name": "db_pass", "value": secret_value})
    _, pub, _ = api(admin, "POST",
                    f"/api/v1/applications/{app['id']}/environments/{env['id']}/publish", {})
    _, group, _ = api(admin, "POST", "/api/v1/node-groups", {"name": "g1"})
    status, asg, _ = api(admin, "POST", "/api/v1/assignments",
                         {"group_id": group["id"], "revision_id": pub["id"]})
    assert status == 201, asg
    _, cmd, _ = api(admin, "POST", "/api/v1/nodes/install-command", {"name": "fixture-node"})
    token = cmd["command"].split("--token ")[1].split()[0]

    node = {
        "work": work, "config": config, "identity_dir": identity_dir,
        "bundle_dir": bundle_dir, "app_id": app["id"], "env_id": env["id"],
        "secret_value": secret_value, "revision_id": pub["id"],
        "group_id": group["id"], "token": token, "admin": admin,
    }
    enroll = subprocess.run([sys.executable, "-m", "autosecrets_agent.cli", "enroll",
                             "--config", str(config), "--token", token],
                            capture_output=True, text=True)
    assert enroll.returncode == 0, f"enroll failed: {enroll.stderr}"
    api(admin, "POST", f"/api/v1/node-groups/{group['id']}/nodes",
        {"node_id": json.loads((identity_dir / "state.json").read_text())["node_id"]})
    return node


def run_agent(node, *args: str) -> subprocess.CompletedProcess:
    return subprocess.run([sys.executable, "-m", "autosecrets_agent.cli", *args,
                           "--config", str(node["config"])], capture_output=True,
                          text=True)


def test_enroll_and_sync_lands_files(managed_node):
    result = run_agent(managed_node, "sync")
    assert result.returncode == 0, result.stderr
    current = managed_node["bundle_dir"] / managed_node["app_id"] / managed_node["env_id"] / "current"
    secret_file = current / "db_pass"
    assert secret_file.read_text() == managed_node["secret_value"]
    assert stat.S_IMODE(os.stat(secret_file).st_mode) == 0o400


def test_sync_is_idempotent(managed_node):
    first = run_agent(managed_node, "sync")
    second = run_agent(managed_node, "sync")
    assert first.returncode == second.returncode == 0
    current = managed_node["bundle_dir"] / managed_node["app_id"] / managed_node["env_id"] / "current"
    assert (current / "db_pass").read_text() == managed_node["secret_value"]


def test_node_reports_observed_revision(managed_node):
    _, nodes, _ = http_json("GET", managed_node["admin"]["core_url"] + "/api/v1/nodes",
                            cookies=managed_node["admin"]["cookie"])
    assert any(n["observed_revision"] == managed_node["revision_id"] and n["last_result"] == "ok"
               for n in nodes)


def test_wrong_signing_key_rejected(managed_node):
    config = managed_node["config"]
    original = config.read_text()
    config.write_text(original.replace('signing_public_key = "', 'signing_public_key = "AAAA'))
    result = run_agent(managed_node, "sync")
    config.write_text(original)
    assert result.returncode != 0, f"corrupted signing key accepted: {result.stderr}"
