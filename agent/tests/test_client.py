from __future__ import annotations

import base64
import time
from datetime import UTC, datetime, timedelta

from cryptography import x509
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import (
    Ed25519PrivateKey,
    Ed25519PublicKey,
)
from cryptography.x509.oid import NameOID

from autosecrets_agent.client import AgentAPI


def _key_and_cert() -> tuple[str, str]:
    key = Ed25519PrivateKey.generate()
    name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "node:test")])
    now = datetime.now(UTC)
    cert = (
        x509.CertificateBuilder()
        .subject_name(name)
        .issuer_name(name)
        .public_key(key.public_key())
        .serial_number(7)
        .not_valid_before(now - timedelta(minutes=1))
        .not_valid_after(now + timedelta(days=1))
        .sign(key, None)
    )
    key_pem = key.private_bytes(
        serialization.Encoding.PEM,
        serialization.PrivateFormat.PKCS8,
        serialization.NoEncryption(),
    ).decode()
    cert_pem = cert.public_bytes(serialization.Encoding.PEM).decode()
    return cert_pem, key_pem


def test_proof_headers_sign_request_path() -> None:
    cert_pem, key_pem = _key_and_cert()
    api = AgentAPI("https://as.example", cert_pem=cert_pem, key_pem=key_pem)
    try:
        headers = api._proof_headers("GET", "/agent/v1/desired")
    finally:
        api.close()
    ts = headers["X-Autosecrets-Ts"]
    assert abs(int(ts) - int(time.time())) < 5
    raw_cert = base64.b64decode(headers["X-Autosecrets-Cert"])
    assert b"BEGIN CERTIFICATE" in raw_cert
    sig = base64.b64decode(headers["X-Autosecrets-Sig"])
    loaded = serialization.load_pem_private_key(key_pem.encode(), password=None)
    pub = loaded.public_key()
    assert isinstance(pub, Ed25519PublicKey)
    pub.verify(sig, f"{ts}\nGET\n/agent/v1/desired".encode())
