"""Node-local materialization: verify, stage, and atomically switch bundles.

Activation is files-only in the first slice: no service actions. On any
failure the previous revision stays current (Last Known Good behavior).
"""

from __future__ import annotations

import base64
import hashlib
import json
import os
import shutil
import tempfile
from dataclasses import dataclass
from pathlib import Path


class MaterializeError(Exception):
    pass


@dataclass
class PayloadFile:
    path: str
    mode: str
    uid: str
    gid: str
    sha256: str
    content: bytes


def parse_payload(plaintext: bytes) -> tuple[str, str, list[PayloadFile]]:
    data = json.loads(plaintext)
    files: list[PayloadFile] = []
    for f in data["files"]:
        content = base64.b64decode(f["content"])
        files.append(PayloadFile(
            path=f["path"], mode=f.get("mode", "0400"),
            uid=f.get("uid", "0"), gid=f.get("gid", "0"),
            sha256=sha256_hex(content), content=content,
        ))
    return data["app_id"], data["env_id"], files


def validate_path(rel: str) -> str:
    """Rejects absolute paths, '..', '.', empty components, and control chars."""
    rel = rel.strip()
    if not rel or rel.startswith("/"):
        raise MaterializeError(f"unsafe path: {rel!r}")
    for part in rel.split("/"):
        if part in ("", ".", ".."):
            raise MaterializeError(f"unsafe path: {rel!r}")
        if len(part.encode()) > 255:
            raise MaterializeError(f"path component too long: {part!r}")
        if any(ord(c) < 0x20 or ord(c) == 0x7F for c in part):
            raise MaterializeError(f"control character in path: {part!r}")
    return rel


def sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _parse_mode(mode: str) -> int:
    try:
        return int(mode, 8)
    except ValueError:
        raise MaterializeError(f"invalid mode: {mode!r}") from None


def materialize(bundle_root: Path, app_id: str, env_id: str,
                revision_id: str, files: list[PayloadFile]) -> None:
    """Writes files to a staging dir inside the bundle dir, verifies content
    hashes, then atomically switches `current` (keeping `previous`)."""
    for f in files:
        validate_path(f.path)
    target = bundle_root / app_id / env_id
    target.mkdir(parents=True, exist_ok=True)
    staging = target / f".staging-{revision_id}"
    if staging.exists():
        shutil.rmtree(staging)

    staging.mkdir(parents=True, exist_ok=False)
    try:
        for f in files:
            dest = staging / f.path
            dest.parent.mkdir(parents=True, exist_ok=True)
            fd, tmp = tempfile.mkstemp(dir=dest.parent, prefix=".write-")
            try:
                with os.fdopen(fd, "wb") as fh:
                    fh.write(f.content)
                os.chmod(tmp, _parse_mode(f.mode))
                if os.geteuid() == 0:
                    os.chown(tmp, int(f.uid), int(f.gid))
                os.replace(tmp, dest)
            finally:
                if os.path.exists(tmp):
                    os.unlink(tmp)
        for f in files:
            actual = sha256_hex((staging / f.path).read_bytes())
            if actual != f.sha256:
                raise MaterializeError(
                    f"content hash mismatch for {f.path}: {actual}")
    except Exception:
        shutil.rmtree(staging, ignore_errors=True)
        raise

    current = target / "current"
    previous = target / "previous"
    if current.exists():
        shutil.rmtree(previous, ignore_errors=True)
        os.replace(current, previous)
    try:
        os.replace(staging, current)
    except Exception:
        # Restore the previous revision as current (Last Known Good).
        if previous.exists() and not current.exists():
            os.replace(previous, current)
        raise
