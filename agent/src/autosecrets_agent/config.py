"""Agent configuration loaded from /etc/autosecrets-agent/config.toml."""

from __future__ import annotations

import tomllib
from dataclasses import dataclass, field
from pathlib import Path

DEFAULT_CONFIG = "/etc/autosecrets-agent/config.toml"
DEFAULT_IDENTITY_DIR = "/var/lib/autosecrets-agent/identity"
DEFAULT_BUNDLE_DIR = "/var/lib/autosecrets-agent/bundles"


@dataclass
class AgentConfig:
    server_url: str
    identity_dir: Path = Path(DEFAULT_IDENTITY_DIR)
    bundle_dir: Path = Path(DEFAULT_BUNDLE_DIR)
    name: str = ""
    signing_public_key: str = ""  # base64 raw Ed25519 Core signing key
    ca_bundle: str = ""  # optional PEM bundle for server TLS verification
    poll_interval_seconds: float = 15.0
    extra: dict = field(default_factory=dict)

    @classmethod
    def load(cls, path: str | Path) -> AgentConfig:
        raw = tomllib.loads(Path(path).read_text(encoding="utf-8"))
        return cls(
            server_url=raw["server_url"].rstrip("/"),
            identity_dir=Path(raw.get("identity_dir", DEFAULT_IDENTITY_DIR)),
            bundle_dir=Path(raw.get("bundle_dir", DEFAULT_BUNDLE_DIR)),
            name=raw.get("name", ""),
            signing_public_key=raw.get("signing_public_key", ""),
            ca_bundle=raw.get("ca_bundle", ""),
            poll_interval_seconds=float(raw.get("poll_interval_seconds", 15.0)),
            extra=raw,
        )
