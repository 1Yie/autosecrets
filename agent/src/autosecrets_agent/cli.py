"""AutoSecrets Agent CLI: verify-envelope (phase 0), enroll, sync, serve."""

from __future__ import annotations

import argparse
import base64
import logging
import sys
from datetime import UTC, datetime
from pathlib import Path

from age.keys.agekey import AgePrivateKey

from autosecrets_agent.client import AgentAPI
from autosecrets_agent.config import AgentConfig
from autosecrets_agent.envelope import Envelope
from autosecrets_agent.identity import (
    generate_age_identity,
    generate_mtls_csr,
    load_age_private,
    save_identity,
)
from autosecrets_agent.sync import serve, sync_once


def _verify_envelope(path: str, signing_public: str, recipient_private: str) -> int:
    envelope = Envelope.from_json(Path(path).read_text(encoding="utf-8"))
    verify_pub = base64.b64decode(signing_public)
    identity = AgePrivateKey.from_private_string(recipient_private)
    plaintext = envelope.open(identity, verify_pub, datetime.now(UTC))
    sys.stdout.buffer.write(plaintext)
    return 0


def _enroll(config: AgentConfig, token: str, server_url: str) -> int:
    identity_dir = config.identity_dir
    identity_dir.mkdir(parents=True, exist_ok=True)
    age_private = load_age_private(identity_dir) or generate_age_identity(identity_dir)
    age_public = AgePrivateKey.from_private_string(age_private).public_key().public_string()
    csr_pem, key_pem = generate_mtls_csr(identity_dir)
    api = AgentAPI(server_url, ca_bundle=config.ca_bundle)
    try:
        body = api.enroll(token, config.name, age_public, csr_pem)
    finally:
        api.close()
    save_identity(identity_dir, body["node_id"], age_private,
                  body["cert_pem"], body["ca_pem"], key_pem)
    print(f"enrolled node {body['node_id']} (cert expires {body['expires_at']})")
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        prog="autosecrets-agent", description="AutoSecrets managed node agent")
    sub = parser.add_subparsers(dest="command", required=True)

    verify = sub.add_parser("verify-envelope", help="verify an envelope and print its plaintext")
    verify.add_argument("envelope_file")
    verify.add_argument("--signing-public", required=True, help="base64 Ed25519 public key")
    verify.add_argument("--recipient-private", required=True, help="age secret key (AGE-SECRET-KEY-1...)")

    def add_config_args(p: argparse.ArgumentParser) -> None:
        p.add_argument("--config", default="/etc/autosecrets-agent/config.toml")
        p.add_argument("--server", default="", help="override server URL (used by the installer)")
        p.add_argument("--token", default="", help="one-time enrollment token (enroll only)")

    enroll = sub.add_parser("enroll", help="enroll this node with a one-time token")
    add_config_args(enroll)

    sync = sub.add_parser("sync", help="run one convergence pass")
    add_config_args(sync)

    servep = sub.add_parser("serve", help="poll and converge continuously")
    add_config_args(servep)
    servep.add_argument("--once", action="store_true", help="single pass, then exit")

    args = parser.parse_args(argv)
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")

    if args.command == "verify-envelope":
        return _verify_envelope(args.envelope_file, args.signing_public, args.recipient_private)

    config = AgentConfig.load(args.config)
    if args.server:
        config.server_url = args.server.rstrip("/")
    if args.token:
        config.extra["token"] = args.token
    if args.command == "enroll":
        if not args.token:
            print("enroll requires --token", file=sys.stderr)
            return 2
        return _enroll(config, args.token, config.server_url)
    if args.command == "sync":
        try:
            result = sync_once(config)
            return 1 if result.get("failed") else 0
        except Exception as e:  # noqa: BLE001
            print(f"sync failed: {e}", file=sys.stderr)
            return 1
    if args.command == "serve":
        return serve(config, once=args.once)
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
