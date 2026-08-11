"""Unit tests for the files-only materializer (slice-1 activation)."""

from __future__ import annotations

import os
import stat

import pytest

from autosecrets_agent.materializer import (
    MaterializeError,
    PayloadFile,
    materialize,
    parse_payload,
    validate_path,
)


def f(path: str, content: bytes = b"value", mode: str = "0400") -> PayloadFile:
    import hashlib
    return PayloadFile(path=path, mode=mode, uid="0", gid="0",
                       sha256=hashlib.sha256(content).hexdigest(), content=content)


class TestValidatePath:
    def test_accepts_relative_nested(self):
        assert validate_path("app/token") == "app/token"

    @pytest.mark.parametrize("bad", ["/etc/passwd", "../escape", "a/../b",
                                     "a//b", "a/./b", "a/\x00b", ""])
    def test_rejects_unsafe(self, bad):
        with pytest.raises(MaterializeError):
            validate_path(bad)


class TestParsePayload:
    def test_roundtrip(self):
        import base64
        import json
        raw = json.dumps({
            "app_id": "app-1", "env_id": "env-1",
            "files": [{"path": "db", "mode": "0600", "uid": "1000", "gid": "1000",
                       "content": base64.b64encode(b"hi").decode()}],
        }).encode()
        app_id, env_id, files = parse_payload(raw)
        assert (app_id, env_id) == ("app-1", "env-1")
        assert files[0].content == b"hi" and files[0].mode == "0600"


class TestMaterialize:
    def test_atomic_switch_and_mode(self, tmp_path):
        bundle = tmp_path / "bundles"
        materialize(bundle, "app-1", "env-1", "rev-1", [f("db_pass", b"secret-1")])
        current = bundle / "app-1" / "env-1" / "current"
        assert (current / "db_pass").read_bytes() == b"secret-1"
        assert stat.S_IMODE(os.stat(current / "db_pass").st_mode) == 0o400
        assert not (bundle / "app-1" / "env-1" / ".staging-rev-1").exists()

        # Second revision switches current → previous, new content current.
        materialize(bundle, "app-1", "env-1", "rev-2", [f("db_pass", b"secret-2")])
        assert (current / "db_pass").read_bytes() == b"secret-2"
        previous = bundle / "app-1" / "env-1" / "previous"
        assert (previous / "db_pass").read_bytes() == b"secret-1"

    def test_hash_mismatch_keeps_previous(self, tmp_path):
        bundle = tmp_path / "bundles"
        materialize(bundle, "app-1", "env-1", "rev-1", [f("x", b"good")])
        bad = f("x", b"tampered")
        bad.sha256 = "0" * 64
        with pytest.raises(MaterializeError):
            materialize(bundle, "app-1", "env-1", "rev-2", [bad])
        current = bundle / "app-1" / "env-1" / "current"
        assert (current / "x").read_bytes() == b"good"
        assert not (bundle / "app-1" / "env-1" / ".staging-rev-2").exists()

    def test_unsafe_path_rejected_before_write(self, tmp_path):
        bundle = tmp_path / "bundles"
        with pytest.raises(MaterializeError):
            materialize(bundle, "app-1", "env-1", "rev-1",
                        [f("../../escape", b"x")])
        assert not (bundle / "app-1").exists()
