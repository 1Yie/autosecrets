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
    def test_flat_layout_and_mode(self, tmp_path):
        bundle = tmp_path / "bundles"
        materialize(bundle, [f("AI/LLM/API", b"secret-1")])
        target = bundle / "AI" / "LLM" / "API"
        assert target.read_bytes() == b"secret-1"
        assert stat.S_IMODE(os.stat(target).st_mode) == 0o400

        # Second revision overwrites the flat path; no current/previous tree.
        materialize(bundle, [f("AI/LLM/API", b"secret-2")])
        assert target.read_bytes() == b"secret-2"
        assert not (bundle / "current").exists()

    def test_hash_mismatch_raises_and_keeps_old_content(self, tmp_path):
        bundle = tmp_path / "bundles"
        materialize(bundle, [f("x", b"good")])
        bad = f("x", b"tampered")
        bad.sha256 = "0" * 64
        with pytest.raises(MaterializeError):
            materialize(bundle, [bad])
        assert (bundle / "x").read_bytes() == b"good"

    def test_unsafe_path_rejected_before_write(self, tmp_path):
        bundle = tmp_path / "bundles"
        with pytest.raises(MaterializeError):
            materialize(bundle, [f("../../escape", b"x")])
        assert not (bundle / ".." / "escape").exists()

    def test_undeclared_files_untouched(self, tmp_path):
        bundle = tmp_path / "bundles"
        legacy = bundle / "AI" / "DeepSeek"
        legacy.parent.mkdir(parents=True)
        legacy.write_bytes(b"legacy-data")
        materialize(bundle, [f("Token", b"new")])
        assert legacy.read_bytes() == b"legacy-data"
        assert (bundle / "Token").read_bytes() == b"new"

    def test_nested_bindings_share_directory(self, tmp_path):
        bundle = tmp_path / "bundles"
        materialize(bundle, [f("A/1", b"one"), f("A/2", b"two"), f("A/3", b"three")])
        assert (bundle / "A" / "1").read_bytes() == b"one"
        assert (bundle / "A" / "2").read_bytes() == b"two"
        assert (bundle / "A" / "3").read_bytes() == b"three"

    def test_flat_file_becomes_directory(self, tmp_path):
        bundle = tmp_path / "bundles"
        # A previous flat binding materialized a FILE at "aaa".
        materialize(bundle, [f("aaa", b"old")])
        assert (bundle / "aaa").is_file()
        # A nested binding "aaa/11" must replace the file with a directory.
        materialize(bundle, [f("aaa/11", b"new")])
        assert (bundle / "aaa").is_dir()
        assert (bundle / "aaa" / "11").read_bytes() == b"new"

    def test_parent_symlink_cannot_escape_bundle_root(self, tmp_path):
        bundle = tmp_path / "bundles"
        outside = tmp_path / "outside"
        bundle.mkdir()
        outside.mkdir()
        (bundle / "linked").symlink_to(outside, target_is_directory=True)

        materialize(bundle, [f("linked/secret", b"inside")])

        assert (bundle / "linked").is_dir()
        assert not (bundle / "linked").is_symlink()
        assert (bundle / "linked" / "secret").read_bytes() == b"inside"
        assert not (outside / "secret").exists()

    def test_cleanup_removes_only_manifest_files(self, tmp_path):
        from autosecrets_agent.materializer import (
            remove_manifest_files,
            save_manifest,
        )
        bundle = tmp_path / "bundles"
        identity_dir = tmp_path / "identity"
        files = [f("a.conf", b"aaa"), f("b.conf", b"bbb")]
        materialize(bundle, files)
        save_manifest(identity_dir, "app-1", "env-1", files)
        foreign = bundle / "foreign.conf"
        foreign.write_bytes(b"keep-me")

        remove_manifest_files(identity_dir, bundle, "app-1", "env-1")

        assert not (bundle / "a.conf").exists()
        assert not (bundle / "b.conf").exists()
        assert foreign.exists(), "files outside the manifest must survive cleanup"
        assert not (identity_dir / "manifests" / "app-1_env-1.json").exists()

    def test_cleanup_skips_parent_symlink_outside_bundle_root(self, tmp_path):
        from autosecrets_agent.materializer import (
            remove_manifest_files,
            save_manifest,
        )
        bundle = tmp_path / "bundles"
        identity_dir = tmp_path / "identity"
        outside = tmp_path / "outside"
        files = [f("linked/secret", b"same-content")]
        materialize(bundle, files)
        save_manifest(identity_dir, "app-1", "env-1", files)
        (bundle / "linked" / "secret").unlink()
        (bundle / "linked").rmdir()
        outside.mkdir()
        external = outside / "secret"
        external.write_bytes(b"same-content")
        (bundle / "linked").symlink_to(outside, target_is_directory=True)

        removed = remove_manifest_files(identity_dir, bundle, "app-1", "env-1")

        assert removed == 0
        assert external.read_bytes() == b"same-content"
