"""Node-local identity: age encryption key, mTLS keypair, and certificate."""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path

from age.keys.agekey import AgePrivateKey
from cryptography import x509
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.x509.oid import NameOID


@dataclass
class NodeIdentity:
    node_id: str
    age_private: str
    age_public: str
    cert_pem: str
    key_pem: str
    ca_pem: str


def _state_path(identity_dir: Path) -> Path:
    return identity_dir / "state.json"


def generate_age_identity(identity_dir: Path) -> str:
    key = AgePrivateKey.generate()
    identity_dir.mkdir(parents=True, exist_ok=True)
    (identity_dir / "age.key").write_text(key.private_string() + "\n", encoding="utf-8")
    return key.private_string()


def load_age_private(identity_dir: Path) -> str:
    path = identity_dir / "age.key"
    if not path.exists():
        return ""
    return path.read_text(encoding="utf-8").strip()


def generate_mtls_csr(identity_dir: Path) -> tuple[str, str]:
    """Returns (csr_pem, key_pem). The key stays node-local (ADR-0006)."""
    key = Ed25519PrivateKey.generate()
    csr = (
        x509.CertificateSigningRequestBuilder()
        .subject_name(x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "autosecrets-agent")]))
        .sign(key, None)  # Ed25519: algorithm must be None
    )
    csr_pem = csr.public_bytes(serialization.Encoding.PEM).decode()
    key_pem = key.private_bytes(
        serialization.Encoding.PEM,
        serialization.PrivateFormat.PKCS8,
        serialization.NoEncryption(),
    ).decode()
    return csr_pem, key_pem


def save_identity(identity_dir: Path, node_id: str, age_private: str,
                  cert_pem: str, ca_pem: str, key_pem: str) -> NodeIdentity:
    identity_dir.mkdir(parents=True, exist_ok=True)
    identity = NodeIdentity(
        node_id=node_id,
        age_private=age_private,
        age_public=AgePrivateKey.from_private_string(age_private).public_key().public_string(),
        cert_pem=cert_pem,
        key_pem=key_pem,
        ca_pem=ca_pem,
    )
    (identity_dir / "age.key").write_text(age_private + "\n", encoding="utf-8")
    (identity_dir / "mtls.key").write_text(key_pem, encoding="utf-8")
    (identity_dir / "cert.pem").write_text(cert_pem, encoding="utf-8")
    (identity_dir / "ca.pem").write_text(ca_pem, encoding="utf-8")
    _state_path(identity_dir).write_text(
        json.dumps({"node_id": node_id}), encoding="utf-8")
    return identity


def load_identity(identity_dir: Path) -> NodeIdentity | None:
    state = _state_path(identity_dir)
    if not state.exists():
        return None
    node_id = json.loads(state.read_text(encoding="utf-8"))["node_id"]
    age_private = load_age_private(identity_dir)
    cert_path, key_path, ca_path = (
        identity_dir / "cert.pem", identity_dir / "mtls.key", identity_dir / "ca.pem")
    if not (age_private and cert_path.exists() and key_path.exists() and ca_path.exists()):
        return None
    return NodeIdentity(
        node_id=node_id,
        age_private=age_private,
        age_public=AgePrivateKey.from_private_string(age_private).public_key().public_string(),
        cert_pem=cert_path.read_text(encoding="utf-8"),
        key_pem=key_path.read_text(encoding="utf-8"),
        ca_pem=ca_path.read_text(encoding="utf-8"),
    )


def save_etag(identity_dir: Path, etag: str) -> None:
    state = json.loads(_state_path(identity_dir).read_text(encoding="utf-8"))
    state["etag"] = etag
    _state_path(identity_dir).write_text(json.dumps(state), encoding="utf-8")


def load_etag(identity_dir: Path) -> str:
    state = _state_path(identity_dir)
    if not state.exists():
        return ""
    return json.loads(state.read_text(encoding="utf-8")).get("etag", "")
