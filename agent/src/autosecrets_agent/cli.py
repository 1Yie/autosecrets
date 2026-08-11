"""Minimal phase-0 command line interface."""

from __future__ import annotations

import argparse
import base64
import sys
from datetime import UTC, datetime
from pathlib import Path

from age.keys.agekey import AgePrivateKey

from autosecrets_agent.envelope import Envelope


def _verify_envelope(path: str, signing_public: str, recipient_private: str) -> int:
    envelope = Envelope.from_json(Path(path).read_text(encoding="utf-8"))
    verify_pub = base64.b64decode(signing_public)
    identity = AgePrivateKey.from_private_string(recipient_private)
    plaintext = envelope.open(identity, verify_pub, datetime.now(UTC))
    sys.stdout.buffer.write(plaintext)
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        prog="autosecrets-agent", description="AutoSecrets managed node agent"
    )
    sub = parser.add_subparsers(dest="command", required=True)
    verify = sub.add_parser("verify-envelope", help="verify an envelope and print its plaintext")
    verify.add_argument("envelope_file")
    verify.add_argument("--signing-public", required=True, help="base64 Ed25519 public key")
    verify.add_argument(
        "--recipient-private", required=True, help="age secret key (AGE-SECRET-KEY-1...)"
    )
    args = parser.parse_args(argv)
    if args.command == "verify-envelope":
        return _verify_envelope(args.envelope_file, args.signing_public, args.recipient_private)
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
