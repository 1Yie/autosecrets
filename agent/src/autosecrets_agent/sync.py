"""Convergence: poll Desired State, verify, decrypt, and materialize.

The polling interval starts from the local config and is re-adjusted on every
pass when Core advertises a per-node `poll_interval_seconds` in the desired or
heartbeat responses."""

from __future__ import annotations

import base64
import json
import logging
import os
import random
import shutil
import subprocess
import time
from datetime import UTC, datetime

from age.keys.agekey import AgePrivateKey

from autosecrets_agent.client import AgentAPI
from autosecrets_agent.config import AgentConfig
from autosecrets_agent.envelope import Envelope
from autosecrets_agent.identity import (
    load_etag,
    load_identity,
    save_etag,
)
from autosecrets_agent.materializer import (
    materialize,
    parse_payload,
    remove_manifest_files,
    save_manifest,
)

log = logging.getLogger("autosecrets-agent")

# Server-side poll interval bounds, mirrored from Core's CHECK constraint.
MIN_POLL_INTERVAL = 5.0
MAX_POLL_INTERVAL = 86400.0


def _interval_from(body: dict) -> float | None:
    """Extract a valid server-provided poll interval, or None."""
    if not isinstance(body, dict):
        return None
    value = body.get("poll_interval_seconds")
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        return None
    try:
        seconds = float(value)
    except (TypeError, ValueError):
        return None
    if MIN_POLL_INTERVAL <= seconds <= MAX_POLL_INTERVAL:
        return seconds
    return None


def _verify_key(config: AgentConfig) -> bytes:
    if not config.signing_public_key:
        raise RuntimeError("signing_public_key is not configured")
    return base64.b64decode(config.signing_public_key)


def _agent_api(config: AgentConfig) -> AgentAPI:
    identity = load_identity(config.identity_dir)
    if identity is None:
        raise RuntimeError(
            "agent is not enrolled; run 'autosecrets-agent enroll' first"
        )
    return AgentAPI(
        config.server_url,
        ca_bundle=config.ca_bundle,
        cert_pem=identity.cert_pem,
        key_pem=identity.key_pem,
    )


def _activate_envelope(
    config: AgentConfig, env: dict, identity, verify_key: bytes
) -> str:
    """Verifies, decrypts, and materializes one envelope; returns revision id."""
    parsed = Envelope.from_json(json.dumps(env))
    age_identity = AgePrivateKey.from_private_string(identity.age_private)
    plaintext = parsed.open(age_identity, verify_key, datetime.now(UTC))
    _app_id, _env_id, files = parse_payload(plaintext)
    from autosecrets_agent.envelope import canonical_manifest

    manifest = canonical_manifest(
        [
            {
                "gid": f.gid,
                "mode": f.mode,
                "path": f.path,
                "sha256": f.sha256,
                "uid": f.uid,
            }
            for f in files
        ]
    )
    parsed.verify_manifest(manifest)
    materialize(config.bundle_dir, files)
    save_manifest(config.identity_dir, _app_id, _env_id, files)
    return parsed.revision_id


def _process_cleanup(
    config: AgentConfig, identity, api: AgentAPI, instructions: list[dict]
) -> None:
    """Executes pending cleanup before any new Desired State: stops the
    Activation Policy units in reverse order, removes the Materialized
    Bundle files, and acknowledges the result per Assignment."""
    for instruction in instructions:
        assignment_id = instruction.get("assignment_id", "")
        app_id = instruction.get("application_id", "")
        env_id = instruction.get("environment_id", "")
        try:
            if shutil.which("systemctl") and not os.environ.get(
                "AUTOSECRETS_NO_SYSTEMD"
            ):
                for unit in reversed(instruction.get("units") or []):
                    stop = subprocess.run(
                        ["systemctl", "stop", unit],
                        capture_output=True,
                        timeout=60,
                        check=False,
                    )
                    if stop.returncode != 0:
                        # A declared-but-absent or already-inactive unit must
                        # not block cleanup; only an actually running unit does.
                        active = subprocess.run(
                            ["systemctl", "is-active", unit],
                            capture_output=True,
                            text=True,
                            timeout=30,
                            check=False,
                        )
                        if active.stdout.strip() in (
                            "active",
                            "activating",
                            "reloading",
                        ):
                            raise RuntimeError(
                                f"unit {unit} is still {active.stdout.strip()}: "
                                f"{stop.stderr.decode(errors='replace')[:200]}"
                            )
            remove_manifest_files(
                config.identity_dir, config.bundle_dir, app_id, env_id
            )
            api.cleanup(identity.node_id, assignment_id, "cleaned")
        except Exception as e:  # noqa: BLE001
            log.error("cleanup failed for %s: %s", assignment_id, e)
            try:
                api.cleanup(identity.node_id, assignment_id, "failed", str(e)[:500])
            except Exception as report_error:  # noqa: BLE001
                log.warning(
                    "failed to report cleanup failure for %s: %s",
                    assignment_id,
                    report_error,
                )


def sync_once(config: AgentConfig) -> dict:
    """One convergence pass. Returns {"changed", "revision", "failed"} plus an
    optional server-side "poll_interval_seconds" when Core advertises one."""
    identity = load_identity(config.identity_dir)
    if identity is None:
        raise RuntimeError(
            "agent is not enrolled; run 'autosecrets-agent enroll' first"
        )
    verify_key = _verify_key(config)
    api = _agent_api(config)
    try:
        etag = load_etag(config.identity_dir)
        status, body = api.desired(etag=etag)
        interval = _interval_from(body)
        if status == 304:
            interval = _heartbeat_interval(api, identity.node_id) or interval
            return {"changed": False, "revision": "", "poll_interval_seconds": interval}
        save_etag(config.identity_dir, body.get("etag", ""))
        _process_cleanup(config, identity, api, body.get("cleanup") or [])
        last_revision = ""
        failed = 0
        for env in body.get("envelopes", []):
            try:
                revision = _activate_envelope(config, env, identity, verify_key)
            except Exception as e:  # noqa: BLE001
                log.error("activation failed for %s: %s", env.get("revision_id"), e)
                api.report(
                    identity.node_id,
                    env.get("revision_id", ""),
                    "activate",
                    "failed",
                    str(e)[:500],
                )
                failed += 1
                continue
            api.report(identity.node_id, revision, "activate", "ok")
            last_revision = revision
        interval = _heartbeat_interval(api, identity.node_id) or interval
        return {
            "changed": bool(body.get("envelopes")),
            "revision": last_revision,
            "failed": failed,
            "poll_interval_seconds": interval,
        }
    finally:
        api.close()


def _heartbeat_interval(api: AgentAPI, node_id: str) -> float | None:
    """Send the liveness heartbeat and read the poll interval it returns."""
    try:
        return _interval_from(api.heartbeat(node_id))
    except Exception as e:  # noqa: BLE001
        log.warning("heartbeat failed: %s", e)
        return None


def serve(config: AgentConfig, once: bool = False) -> int:
    try:
        interval = float(config.poll_interval_seconds)
    except (TypeError, ValueError):
        interval = MIN_POLL_INTERVAL
    interval = max(MIN_POLL_INTERVAL, interval)
    while True:
        try:
            result = sync_once(config)
            server_interval = result.get("poll_interval_seconds")
            if server_interval is not None:
                interval = server_interval
            log.info(
                "sync: changed=%s revision=%s interval=%ss",
                result["changed"],
                result["revision"],
                interval,
            )
        except Exception as e:  # noqa: BLE001
            log.error("sync failed: %s", e)
        if once:
            return 0
        time.sleep(interval + random.uniform(0, 3))
