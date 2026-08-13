"""Node-local materialization: write Secret files flat under the bundle root.

The layout matches the legacy tool: a File Binding path such as
"AI/LLM/API" lands directly at <bundle_root>/AI/LLM/API (default bundle
root: ~/.autosecrets). Every file is written via a temp file + atomic
rename, so an interrupted write never leaves a torn file. Files not
declared by the delivered revision are left untouched: the directory may
also hold data deployed by the legacy tool, which must never be wiped.
"""

from __future__ import annotations

import base64
import hashlib
import json
import os
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


def _ensure_dir(bundle_root: Path, path: Path) -> None:
    """Create real directories below the bundle root.

    Existing files and symlinks are replaced so a local symlink cannot redirect
    a signed relative binding outside the configured Materialized Bundle.
    """
    current = bundle_root
    for part in path.relative_to(bundle_root).parts:
        current /= part
        if current.is_symlink() or (current.exists() and not current.is_dir()):
            current.unlink()
        current.mkdir(exist_ok=True)


def _manifest_target(bundle_root: Path, rel: str) -> Path | None:
    """Resolve a manifest path only while every parent remains a real directory."""
    validate_path(rel)
    parts = Path(rel).parts
    current = bundle_root
    for part in parts[:-1]:
        current /= part
        if current.is_symlink() or not current.is_dir():
            return None
    target = current / parts[-1]
    return None if target.is_symlink() else target


def materialize(bundle_root: Path, files: list[PayloadFile]) -> None:
    """Writes files flat under bundle_root with per-file atomic replace,
    then verifies content hashes. On failure the error propagates and the
    agent reports it; the next convergence pass retries."""
    for f in files:
        validate_path(f.path)
    # Verify every content hash in memory BEFORE touching the disk, so a
    # tampered payload can never overwrite a good file.
    for f in files:
        if sha256_hex(f.content) != f.sha256:
            raise MaterializeError(
                f"content hash mismatch for {f.path}: "
                f"expected {f.sha256}, got {sha256_hex(f.content)}")
    bundle_root.mkdir(parents=True, exist_ok=True)
    # Partial writes from a failed pass stay in place; the next pass
    # overwrites them, and pre-existing files are never touched.
    for f in files:
        dest = bundle_root / f.path
        _ensure_dir(bundle_root, dest.parent)
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


def _manifest_path(identity_dir: Path, app_id: str, env_id: str) -> Path:
    return identity_dir / "manifests" / f"{app_id}_{env_id}.json"


def save_manifest(identity_dir: Path, app_id: str, env_id: str,
                  files: list[PayloadFile]) -> None:
    """Records which files the last Activation wrote for one Bundle, so
    Unassignment cleanup can remove exactly the delivered set."""
    path = _manifest_path(identity_dir, app_id, env_id)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps([
        {"path": f.path, "sha256": f.sha256} for f in files
    ]), encoding="utf-8")


def remove_manifest_files(identity_dir: Path, bundle_root: Path,
                          app_id: str, env_id: str) -> int:
    """Deletes the files recorded in the Bundle manifest (verifying their
    content hash first) and then the manifest itself. Files the Agent never
    wrote — anything outside the manifest — are left untouched."""
    path = _manifest_path(identity_dir, app_id, env_id)
    if not path.exists():
        return 0
    manifest = json.loads(path.read_text(encoding="utf-8"))
    removed = 0
    for entry in manifest:
        try:
            target = _manifest_target(bundle_root, entry["path"])
            if target is None:
                continue
            if target.is_file() and sha256_hex(target.read_bytes()) == entry["sha256"]:
                target.unlink()
                removed += 1
        except (MaterializeError, OSError):
            # A file the Agent no longer controls is not a cleanup failure
            # the control plane can resolve; keep going.
            continue
    path.unlink(missing_ok=True)
    return removed
