"""Versioned Agent envelope protocol v1.

Mirror of the Go reference implementation in core/internal/envelope. Both
implementations must produce identical canonical JSON bytes (sorted keys, no
whitespace, Go-compatible string escaping) so signatures and manifests verify
across languages. See api/agent-envelope/envelope-v1.md.
"""

from __future__ import annotations

import base64
import hashlib
import io
import json
from dataclasses import asdict, dataclass
from datetime import UTC, datetime
from typing import Any

from age.file import Decryptor, Encryptor
from age.keys.agekey import AgePrivateKey, AgePublicKey
from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives.asymmetric.ed25519 import (
    Ed25519PrivateKey,
    Ed25519PublicKey,
)

PROTOCOL_NAME = "autosecrets-envelope"
VERSION = 1
SUITE_AGE_X25519 = "age-x25519"

_RFC3339_UTC = "%Y-%m-%dT%H:%M:%SZ"


class EnvelopeError(Exception):
    """Base class for all envelope failures. Callers must treat these as
    fail-closed outcomes, never as a signal to continue."""


class UnsupportedProtocolError(EnvelopeError):
    pass


class UnsupportedVersionError(EnvelopeError):
    pass


class UnsupportedSuiteError(EnvelopeError):
    pass


class ExpiredError(EnvelopeError):
    pass


class BadSignatureError(EnvelopeError):
    pass


class CiphertextError(EnvelopeError):
    pass


class BadManifestError(EnvelopeError):
    pass


class CanonicalError(EnvelopeError):
    pass


def canonical_json(obj: Any) -> str:
    """Serialize like Go's encoding/json: sorted keys, no whitespace, raw
    non-ASCII bytes, and <, >, &, U+2028, U+2029 escaped exactly as Go does."""
    text = json.dumps(obj, sort_keys=True, separators=(",", ":"), ensure_ascii=False)
    return (
        text.replace("<", "\\u003c")
        .replace(">", "\\u003e")
        .replace("&", "\\u0026")
        .replace("\u2028", "\\u2028")
        .replace("\u2029", "\\u2029")
    )


def canonical_manifest(files: list[dict[str, str]]) -> bytes:
    """Serialize FileSpecs in the canonical manifest form."""
    try:
        entries = [dict(f) for f in sorted(files, key=lambda f: f["path"])]
        for entry in entries:
            for key in ("gid", "mode", "path", "sha256", "uid"):
                if key not in entry:
                    raise KeyError(key)
    except (KeyError, TypeError) as exc:
        raise CanonicalError(f"invalid file spec: {exc}") from exc
    manifest = {"files": entries, "protocol": "autosecrets-manifest", "version": "1"}
    return canonical_json(manifest).encode()


@dataclass
class Envelope:
    protocol: str
    version: int
    node_id: str
    revision_id: str
    created_at: str
    expires_at: str
    manifest_sha256: str
    suite: str
    ciphertext: str
    signing_key_id: str
    signature: str

    @classmethod
    def from_json(cls, data: str | bytes) -> Envelope:
        obj = json.loads(data)
        return cls(**obj)

    def to_json(self) -> str:
        return json.dumps(asdict(self), sort_keys=True, separators=(",", ":"))

    def signature_payload(self) -> bytes:
        fields = {
            "ciphertext": self.ciphertext,
            "created_at": self.created_at,
            "expires_at": self.expires_at,
            "manifest_sha256": self.manifest_sha256,
            "node_id": self.node_id,
            "protocol": self.protocol,
            "revision_id": self.revision_id,
            "signing_key_id": self.signing_key_id,
            "suite": self.suite,
            "version": str(self.version),
        }
        return canonical_json(fields).encode()

    def _check_meta(self) -> None:
        if self.protocol != PROTOCOL_NAME:
            raise UnsupportedProtocolError(f"unsupported protocol {self.protocol!r}")
        if self.version != VERSION:
            raise UnsupportedVersionError(f"unsupported version {self.version!r}")
        if self.suite != SUITE_AGE_X25519:
            raise UnsupportedSuiteError(f"unsupported suite {self.suite!r}")

    def _check_expiry(self, now: datetime) -> None:
        if not self.expires_at:
            return
        try:
            expiry = datetime.strptime(self.expires_at, _RFC3339_UTC).replace(tzinfo=UTC)
        except ValueError as exc:
            raise EnvelopeError(f"invalid expires_at {self.expires_at!r}") from exc
        if now > expiry:
            raise ExpiredError(f"envelope expired at {self.expires_at}")

    def verify(self, verify_pub: bytes, now: datetime) -> None:
        self._check_meta()
        self._check_expiry(now)
        try:
            signature = base64.b64decode(self.signature, validate=True)
        except (ValueError, TypeError) as exc:
            raise BadSignatureError("signature is not valid base64") from exc
        try:
            public_key = Ed25519PublicKey.from_public_bytes(verify_pub)
            public_key.verify(signature, self.signature_payload())
        except (InvalidSignature, ValueError) as exc:
            raise BadSignatureError("signature does not verify") from exc

    def open(self, identity: AgePrivateKey, verify_pub: bytes, now: datetime) -> bytes:
        self.verify(verify_pub, now)
        try:
            ciphertext = base64.b64decode(self.ciphertext, validate=True)
        except (ValueError, TypeError) as exc:
            raise CiphertextError("ciphertext is not valid base64") from exc
        try:
            decryptor = Decryptor([identity], io.BytesIO(ciphertext))
            return decryptor.read()
        except Exception as exc:
            raise CiphertextError("ciphertext could not be decrypted") from exc

    def verify_manifest(self, manifest: bytes) -> None:
        digest = hashlib.sha256(manifest).hexdigest()
        if digest != self.manifest_sha256:
            raise BadManifestError("manifest hash mismatch")


def _format_utc(value: datetime | None) -> str:
    if value is None:
        return ""
    return value.astimezone(UTC).strftime(_RFC3339_UTC)


def create(
    *,
    plaintext: bytes,
    manifest: bytes,
    recipient_pub: AgePublicKey,
    signer: Ed25519PrivateKey,
    node_id: str,
    revision_id: str,
    created_at: datetime | None,
    expires_at: datetime | None,
    signing_key_id: str,
) -> Envelope:
    if not manifest:
        raise EnvelopeError("manifest required")
    if created_at is None:
        created_at = datetime.now(UTC)

    buffer = io.BytesIO()
    with Encryptor([recipient_pub], buffer) as encryptor:
        encryptor.write(plaintext)

    manifest_sha256 = hashlib.sha256(manifest).hexdigest()
    envelope = Envelope(
        protocol=PROTOCOL_NAME,
        version=VERSION,
        node_id=node_id,
        revision_id=revision_id,
        created_at=_format_utc(created_at),
        expires_at=_format_utc(expires_at),
        manifest_sha256=manifest_sha256,
        suite=SUITE_AGE_X25519,
        ciphertext=base64.b64encode(buffer.getvalue()).decode(),
        signing_key_id=signing_key_id,
        signature="",
    )
    envelope.signature = base64.b64encode(signer.sign(envelope.signature_payload())).decode()
    return envelope
