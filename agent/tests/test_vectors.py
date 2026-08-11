import base64
import json
from datetime import UTC, datetime
from pathlib import Path

import pytest

from autosecrets_agent.envelope import (
    BadSignatureError,
    Envelope,
    ExpiredError,
)

REPO_ROOT = Path(__file__).resolve().parents[2]
VECTORS = REPO_ROOT / "api" / "agent-envelope" / "testdata" / "envelope-v1-vectors.json"


def load_vectors():
    with open(VECTORS, encoding="utf-8") as f:
        data = json.load(f)
    assert data["protocol"] == "autosecrets-envelope"
    assert data["version"] == 1
    return data["vectors"]


def open_vector(v, now):
    from age.keys.agekey import AgePrivateKey

    identity = AgePrivateKey.from_private_string(v["recipient_private"])
    verify_pub = base64.b64decode(v["signing_public"])
    env = Envelope.from_json(json.dumps(v["envelope"]))
    return env.open(identity, verify_pub, now)


@pytest.mark.parametrize("v", load_vectors(), ids=lambda v: v["name"])
def test_vectors_from_both_implementations(v):
    now = datetime(2026, 8, 11, 12, tzinfo=UTC)
    kind = v.get("must_fail", "")
    if kind == "":
        plaintext = open_vector(v, now)
        assert plaintext == base64.b64decode(v["plaintext"])
        env = Envelope.from_json(json.dumps(v["envelope"]))
        env.verify_manifest(v["manifest"].encode())
    elif kind == "signature":
        with pytest.raises(BadSignatureError):
            open_vector(v, now)
    elif kind == "expired":
        with pytest.raises(ExpiredError):
            open_vector(v, now)
    else:
        raise AssertionError(f"unknown must_fail kind {kind}")
