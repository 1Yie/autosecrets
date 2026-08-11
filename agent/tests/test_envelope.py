import base64
from datetime import UTC, datetime

import pytest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from autosecrets_agent.envelope import (
    PROTOCOL_NAME,
    SUITE_AGE_X25519,
    VERSION,
    BadManifestError,
    BadSignatureError,
    CiphertextError,
    Envelope,
    ExpiredError,
    UnsupportedProtocolError,
    UnsupportedSuiteError,
    UnsupportedVersionError,
    canonical_json,
    canonical_manifest,
    create,
)


def make_keys():
    from age.keys.agekey import AgePrivateKey

    identity = AgePrivateKey.generate()
    return identity


def test_round_trip():
    identity = make_keys()
    signer = Ed25519PrivateKey.generate()
    manifest = (
        b'{"files":[{"gid":"0","mode":"0400","path":"app/token",'
        b'"sha256":"abc123","uid":"1000"}],"protocol":"autosecrets-manifest",'
        b'"version":"1"}'
    )
    plaintext = b"super-secret-token-value"
    env = create(
        plaintext=plaintext,
        manifest=manifest,
        recipient_pub=identity.public_key(),
        signer=signer,
        node_id="node-1",
        revision_id="rev-42",
        created_at=datetime(2026, 8, 11, tzinfo=UTC),
        expires_at=datetime(2026, 8, 12, tzinfo=UTC),
        signing_key_id="core-signing-1",
    )
    verify_pub = signer.public_key().public_bytes_raw()
    got = env.open(identity, verify_pub, datetime(2026, 8, 11, 12, tzinfo=UTC))
    assert got == plaintext
    env.verify_manifest(manifest)


def test_round_trip_survives_remarshal():
    identity = make_keys()
    signer = Ed25519PrivateKey.generate()
    env = create(
        plaintext=b"x",
        manifest=b'{"files":[],"protocol":"autosecrets-manifest","version":"1"}',
        recipient_pub=identity.public_key(),
        signer=signer,
        node_id="node-1",
        revision_id="rev-42",
        created_at=datetime(2026, 8, 11, tzinfo=UTC),
        expires_at=datetime(2026, 8, 12, tzinfo=UTC),
        signing_key_id="core-signing-1",
    )
    again = Envelope.from_json(env.to_json())
    assert again.open(
        identity, signer.public_key().public_bytes_raw(), datetime(2026, 8, 11, 12, tzinfo=UTC)
    ) == b"x"


def test_expired_rejected():
    identity = make_keys()
    signer = Ed25519PrivateKey.generate()
    env = create(
        plaintext=b"x",
        manifest=b'{"files":[],"protocol":"autosecrets-manifest","version":"1"}',
        recipient_pub=identity.public_key(),
        signer=signer,
        node_id="n",
        revision_id="r",
        created_at=datetime(2025, 1, 1, tzinfo=UTC),
        expires_at=datetime(2025, 2, 1, tzinfo=UTC),
        signing_key_id="k",
    )
    now = datetime(2026, 8, 11, tzinfo=UTC)
    verify_pub = signer.public_key().public_bytes_raw()
    with pytest.raises(ExpiredError):
        env.verify(verify_pub, now)
    with pytest.raises(ExpiredError):
        env.open(identity, verify_pub, now)


def test_no_expiry_allows_future_open():
    identity = make_keys()
    signer = Ed25519PrivateKey.generate()
    env = create(
        plaintext=b"x",
        manifest=b'{"files":[],"protocol":"autosecrets-manifest","version":"1"}',
        recipient_pub=identity.public_key(),
        signer=signer,
        node_id="n",
        revision_id="r",
        created_at=datetime(2026, 8, 11, tzinfo=UTC),
        expires_at=None,
        signing_key_id="k",
    )
    env.open(identity, signer.public_key().public_bytes_raw(), datetime(2100, 1, 1, tzinfo=UTC))


def test_bad_signature_rejected():
    identity = make_keys()
    signer = Ed25519PrivateKey.generate()
    env = create(
        plaintext=b"x",
        manifest=b'{"files":[],"protocol":"autosecrets-manifest","version":"1"}',
        recipient_pub=identity.public_key(),
        signer=signer,
        node_id="n",
        revision_id="r",
        created_at=datetime(2026, 8, 11, tzinfo=UTC),
        expires_at=datetime(2026, 8, 12, tzinfo=UTC),
        signing_key_id="k",
    )
    sig = bytearray(base64.b64decode(env.signature))
    sig[0] ^= 0xFF
    env.signature = base64.b64encode(bytes(sig)).decode()
    with pytest.raises(BadSignatureError):
        env.open(identity, signer.public_key().public_bytes_raw(), datetime(2026, 8, 11, tzinfo=UTC))


def test_wrong_identity_rejected():
    identity = make_keys()
    other = make_keys()
    signer = Ed25519PrivateKey.generate()
    env = create(
        plaintext=b"x",
        manifest=b'{"files":[],"protocol":"autosecrets-manifest","version":"1"}',
        recipient_pub=identity.public_key(),
        signer=signer,
        node_id="n",
        revision_id="r",
        created_at=datetime(2026, 8, 11, tzinfo=UTC),
        expires_at=datetime(2026, 8, 12, tzinfo=UTC),
        signing_key_id="k",
    )
    with pytest.raises(CiphertextError):
        env.open(other, signer.public_key().public_bytes_raw(), datetime(2026, 8, 11, tzinfo=UTC))


def test_wrong_signer_rejected():
    identity = make_keys()
    signer = Ed25519PrivateKey.generate()
    other_signer = Ed25519PrivateKey.generate()
    env = create(
        plaintext=b"x",
        manifest=b'{"files":[],"protocol":"autosecrets-manifest","version":"1"}',
        recipient_pub=identity.public_key(),
        signer=other_signer,
        node_id="n",
        revision_id="r",
        created_at=datetime(2026, 8, 11, tzinfo=UTC),
        expires_at=datetime(2026, 8, 12, tzinfo=UTC),
        signing_key_id="k",
    )
    with pytest.raises(BadSignatureError):
        env.open(identity, signer.public_key().public_bytes_raw(), datetime(2026, 8, 11, tzinfo=UTC))


def test_unsupported_protocol_version_suite():
    identity = make_keys()
    signer = Ed25519PrivateKey.generate()
    env = create(
        plaintext=b"x",
        manifest=b'{"files":[],"protocol":"autosecrets-manifest","version":"1"}',
        recipient_pub=identity.public_key(),
        signer=signer,
        node_id="n",
        revision_id="r",
        created_at=datetime(2026, 8, 11, tzinfo=UTC),
        expires_at=datetime(2026, 8, 12, tzinfo=UTC),
        signing_key_id="k",
    )
    now = datetime(2026, 8, 11, tzinfo=UTC)
    verify_pub = signer.public_key().public_bytes_raw()
    env.protocol = "other"
    with pytest.raises(UnsupportedProtocolError):
        env.open(identity, verify_pub, now)
    env.protocol = PROTOCOL_NAME
    env.version = 2
    with pytest.raises(UnsupportedVersionError):
        env.open(identity, verify_pub, now)
    env.version = VERSION
    env.suite = "aes-gcm"
    with pytest.raises(UnsupportedSuiteError):
        env.open(identity, verify_pub, now)


def test_verify_manifest_mismatch():
    identity = make_keys()
    signer = Ed25519PrivateKey.generate()
    env = create(
        plaintext=b"x",
        manifest=b'{"files":[],"protocol":"autosecrets-manifest","version":"1"}',
        recipient_pub=identity.public_key(),
        signer=signer,
        node_id="n",
        revision_id="r",
        created_at=datetime(2026, 8, 11, tzinfo=UTC),
        expires_at=datetime(2026, 8, 12, tzinfo=UTC),
        signing_key_id="k",
    )
    with pytest.raises(BadManifestError):
        env.verify_manifest(b'{"files":[{"path":"x"}]}')


def test_canonical_signature_payload_matches_go():
    env = Envelope(
        protocol=PROTOCOL_NAME,
        version=1,
        node_id="node-1",
        revision_id="rev-42",
        created_at="2026-08-11T00:00:00Z",
        expires_at="",
        manifest_sha256="deadbeef",
        suite=SUITE_AGE_X25519,
        ciphertext="cipher",
        signing_key_id="core-signing-1",
        signature="",
    )
    want = (
        b'{"ciphertext":"cipher","created_at":"2026-08-11T00:00:00Z",'
        b'"expires_at":"","manifest_sha256":"deadbeef","node_id":"node-1",'
        b'"protocol":"autosecrets-envelope","revision_id":"rev-42",'
        b'"signing_key_id":"core-signing-1","suite":"age-x25519","version":"1"}'
    )
    assert env.signature_payload() == want


def test_canonical_manifest_matches_go():
    got = canonical_manifest(
        [
            {"path": "b", "mode": "0400", "uid": "1", "gid": "1", "sha256": "y"},
            {"path": "a", "mode": "0600", "uid": "2", "gid": "2", "sha256": "x"},
        ]
    )
    want = (
        b'{"files":[{"gid":"2","mode":"0600","path":"a","sha256":"x","uid":"2"},'
        b'{"gid":"1","mode":"0400","path":"b","sha256":"y","uid":"1"}],'
        b'"protocol":"autosecrets-manifest","version":"1"}'
    )
    assert got == want


def test_canonical_json_go_string_escaping():
    # Go's encoding/json escapes <, >, & as \u003c, \u003e, \u0026 but keeps
    # other non-ASCII bytes raw. Python must produce identical bytes.
    got = canonical_json({"a": "应用<x>&"})
    assert got == '{"a":"应用\\u003cx\\u003e\\u0026"}'


def test_canonical_json_u2028_u2029_escaping():
    # Go's encoding/json also escapes U+2028 and U+2029 (JSONP-safe output).
    got = canonical_json({"a": "\u2028\u2029"})
    assert got == '{"a":"\\u2028\\u2029"}'


def test_short_verify_key_raises_bad_signature():
    identity = make_keys()
    signer = Ed25519PrivateKey.generate()
    env = create(
        plaintext=b"x",
        manifest=b'{"files":[],"protocol":"autosecrets-manifest","version":"1"}',
        recipient_pub=identity.public_key(),
        signer=signer,
        node_id="n",
        revision_id="r",
        created_at=datetime(2026, 8, 11, tzinfo=UTC),
        expires_at=datetime(2026, 8, 12, tzinfo=UTC),
        signing_key_id="k",
    )
    with pytest.raises(BadSignatureError):
        env.open(identity, b"too-short", datetime(2026, 8, 11, tzinfo=UTC))


def test_errors_never_contain_plaintext():
    identity = make_keys()
    signer = Ed25519PrivateKey.generate()
    plaintext = b"TOP-SECRET-7f3a9c"
    env = create(
        plaintext=plaintext,
        manifest=b'{"files":[],"protocol":"autosecrets-manifest","version":"1"}',
        recipient_pub=identity.public_key(),
        signer=signer,
        node_id="n",
        revision_id="r",
        created_at=datetime(2026, 8, 11, tzinfo=UTC),
        expires_at=datetime(2026, 8, 12, tzinfo=UTC),
        signing_key_id="k",
    )
    sig = bytearray(base64.b64decode(env.signature))
    sig[0] ^= 0xFF
    env.signature = base64.b64encode(bytes(sig)).decode()
    with pytest.raises(BadSignatureError) as excinfo:
        env.open(identity, signer.public_key().public_bytes_raw(), datetime(2026, 8, 11, tzinfo=UTC))
    assert plaintext not in str(excinfo.value).encode()
