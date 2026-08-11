"""Cross-language interop helper used by the phase-0 risk gate test.

The Go test core/internal/envelope/interop_test.go spawns:

    python -m autosecrets_agent.interop roundtrip INPUT OUTPUT

It decrypts and verifies a Go-produced envelope, then produces a fresh
Python-produced envelope for Go to decrypt and verify.
"""

from __future__ import annotations

import argparse
import base64
import json
from datetime import UTC, datetime
from pathlib import Path

from age.keys.agekey import AgePrivateKey
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from autosecrets_agent.envelope import Envelope, create


def _roundtrip(input_path: Path, output_path: Path) -> None:
    data = json.loads(input_path.read_text(encoding="utf-8"))
    envelope = Envelope.from_json(json.dumps(data["envelope"]))
    identity = AgePrivateKey.from_private_string(data["recipient_private"])
    verify_pub = base64.b64decode(data["signing_public"])
    now = datetime.now(UTC)

    plaintext = envelope.open(identity, verify_pub, now)
    expected = base64.b64decode(data["plaintext"])
    if plaintext != expected:
        raise AssertionError("plaintext mismatch")
    envelope.verify_manifest(data["manifest"].encode())

    py_identity = AgePrivateKey.generate()
    py_signer = Ed25519PrivateKey.generate()
    message = b"python->go round trip payload"
    manifest = (
        b'{"files":[{"gid":"0","mode":"0400","path":"app/token",'
        b'"sha256":"def456","uid":"1001"}],"protocol":"autosecrets-manifest",'
        b'"version":"1"}'
    )
    py_envelope = create(
        plaintext=message,
        manifest=manifest,
        recipient_pub=py_identity.public_key(),
        signer=py_signer,
        node_id="node-py",
        revision_id="rev-py",
        created_at=now,
        expires_at=None,
        signing_key_id="py-signing-1",
    )
    output = {
        "envelope": json.loads(py_envelope.to_json()),
        "recipient_public": py_identity.public_key().public_string(),
        "recipient_private": py_identity.private_string(),
        "signing_public": base64.b64encode(py_signer.public_key().public_bytes_raw()).decode(),
        "signing_private": "",
        "manifest": manifest.decode(),
        "plaintext": base64.b64encode(message).decode(),
    }
    output_path.write_text(json.dumps(output, indent=2), encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(prog="autosecrets-agent-interop")
    sub = parser.add_subparsers(dest="command", required=True)
    rt = sub.add_parser("roundtrip", help="verify a Go envelope and produce a Python envelope")
    rt.add_argument("input")
    rt.add_argument("output")
    args = parser.parse_args()
    if args.command == "roundtrip":
        _roundtrip(Path(args.input), Path(args.output))
        return 0
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
