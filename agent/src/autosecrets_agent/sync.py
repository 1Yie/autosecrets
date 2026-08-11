"""Convergence: poll Desired State, verify, decrypt, and materialize."""

from __future__ import annotations

import base64
import json
import logging
import random
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
)

log = logging.getLogger("autosecrets-agent")


def _verify_key(config: AgentConfig) -> bytes:
    if not config.signing_public_key:
        raise RuntimeError("signing_public_key is not configured")
    return base64.b64decode(config.signing_public_key)


def _agent_api(config: AgentConfig) -> AgentAPI:
    identity = load_identity(config.identity_dir)
    if identity is None:
        raise RuntimeError("agent is not enrolled; run 'autosecrets-agent enroll' first")
    return AgentAPI(
        config.server_url,
        ca_bundle=config.ca_bundle,
        cert_pem=identity.cert_pem,
        key_pem=identity.key_pem,
    )


def _activate_envelope(config: AgentConfig, env: dict, identity, verify_key: bytes) -> str:
    """Verifies, decrypts, and materializes one envelope; returns revision id."""
    parsed = Envelope.from_json(json.dumps(env))
    age_identity = AgePrivateKey.from_private_string(identity.age_private)
    plaintext = parsed.open(age_identity, verify_key, datetime.now(UTC))
    app_id, env_id, files = parse_payload(plaintext)
    from autosecrets_agent.envelope import canonical_manifest

    manifest = canonical_manifest([
        {"gid": f.gid, "mode": f.mode, "path": f.path,
         "sha256": f.sha256, "uid": f.uid}
        for f in files
    ])
    parsed.verify_manifest(manifest)
    materialize(config.bundle_dir, app_id, env_id, parsed.revision_id, files)
    return parsed.revision_id


def sync_once(config: AgentConfig) -> dict:
    """One convergence pass. Returns {"changed", "revision", "failed"}."""
    identity = load_identity(config.identity_dir)
    if identity is None:
        raise RuntimeError("agent is not enrolled; run 'autosecrets-agent enroll' first")
    verify_key = _verify_key(config)
    api = _agent_api(config)
    try:
        etag = load_etag(config.identity_dir)
        status, body = api.desired(etag=etag)
        if status == 304:
            api.heartbeat(identity.node_id)
            return {"changed": False, "revision": ""}
        save_etag(config.identity_dir, body.get("etag", ""))
        last_revision = ""
        failed = 0
        for env in body.get("envelopes", []):
            try:
                revision = _activate_envelope(config, env, identity, verify_key)
            except Exception as e:  # noqa: BLE001
                log.error("activation failed for %s: %s", env.get("revision_id"), e)
                api.report(identity.node_id, env.get("revision_id", ""),
                           "activate", "failed", str(e)[:500])
                failed += 1
                continue
            api.report(identity.node_id, revision, "activate", "ok")
            last_revision = revision
        api.heartbeat(identity.node_id)
        return {"changed": bool(body.get("envelopes")), "revision": last_revision,
                "failed": failed}
    finally:
        api.close()


def serve(config: AgentConfig, once: bool = False) -> int:
    interval = max(1.0, config.poll_interval_seconds)
    while True:
        try:
            result = sync_once(config)
            log.info("sync: changed=%s revision=%s", result["changed"], result["revision"])
        except Exception as e:  # noqa: BLE001
            log.error("sync failed: %s", e)
        if once:
            return 0
        time.sleep(interval + random.uniform(0, 3))
