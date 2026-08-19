"""HTTP client for the Agent API: plain HTTPS for enrollment, mTLS after."""

from __future__ import annotations

import base64
import json
import ssl
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any


class AgentAPIError(Exception):
    def __init__(self, status: int, body: str):
        super().__init__(f"agent API {status}: {body[:300]}")
        self.status = status
        self.body = body


class AgentAPI:
    def __init__(
        self,
        server_url: str,
        ca_bundle: str = "",
        cert_pem: str = "",
        key_pem: str = "",
    ):
        self.base = server_url.rstrip("/")
        self._cert_pem = cert_pem
        self._key_pem = key_pem
        if ca_bundle and Path(ca_bundle).exists():
            ctx = ssl.create_default_context(cafile=ca_bundle)
        else:
            # No bundle yet (pre-enrollment) or system trust: default context.
            ctx = ssl.create_default_context()
        if cert_pem and key_pem:
            # Client certificate for mTLS (the proxy terminates TLS and
            # forwards the serial to Core; the Agent never sets that header).
            import tempfile

            with (
                tempfile.NamedTemporaryFile("w", suffix=".pem", delete=False) as c,
                tempfile.NamedTemporaryFile("w", suffix=".pem", delete=False) as k,
            ):
                c.write(cert_pem)
                k.write(key_pem)
                self._cert_file, self._key_file = c.name, k.name
            ctx.load_cert_chain(self._cert_file, self._key_file)
        self.ctx = ctx

    def close(self) -> None:
        for attr in ("_cert_file", "_key_file"):
            if hasattr(self, attr):
                Path(getattr(self, attr)).unlink(missing_ok=True)

    def _proof_headers(self, method: str, path: str) -> dict[str, str]:
        if not self._cert_pem or not self._key_pem:
            return {}
        from cryptography.hazmat.primitives import serialization
        from cryptography.hazmat.primitives.asymmetric.ed25519 import (
            Ed25519PrivateKey,
        )

        ts = str(int(time.time()))
        message = f"{ts}\n{method}\n{path}".encode()
        try:
            loaded = serialization.load_pem_private_key(
                self._key_pem.encode(), password=None
            )
        except (ValueError, TypeError) as e:
            raise RuntimeError("agent key is not a valid PEM private key") from e
        if not isinstance(loaded, Ed25519PrivateKey):
            raise RuntimeError("agent key is not Ed25519")
        sig = loaded.sign(message)
        return {
            "X-Autosecrets-Cert": base64.b64encode(self._cert_pem.encode()).decode(),
            "X-Autosecrets-Ts": ts,
            "X-Autosecrets-Sig": base64.b64encode(sig).decode(),
        }

    def _request(
        self,
        method: str,
        path: str,
        body: Any | None = None,
        headers: dict[str, str] | None = None,
    ) -> tuple[int, dict[str, Any]]:
        data = None
        merged = {**self._proof_headers(method, path), **(headers or {})}
        if body is not None:
            data = json.dumps(body).encode()
            merged["Content-Type"] = "application/json"
        req = urllib.request.Request(
            self.base + path, data=data, method=method, headers=merged
        )
        opener = urllib.request.build_opener(
            urllib.request.ProxyHandler({}),
            urllib.request.HTTPSHandler(context=self.ctx),
        )
        try:
            with opener.open(req, timeout=30) as resp:
                raw = resp.read()
                return resp.status, (json.loads(raw) if raw else {})
        except urllib.error.HTTPError as e:
            raw = e.read().decode("utf-8", "replace")
            raise AgentAPIError(e.code, raw) from None

    def enroll(
        self, token: str, name: str, age_pubkey: str, csr: str
    ) -> dict[str, Any]:
        status, body = self._request(
            "POST",
            "/agent/v1/enroll",
            {
                "token": token,
                "name": name,
                "age_pubkey": age_pubkey,
                "csr": csr,
            },
        )
        if status != 201:
            raise AgentAPIError(status, json.dumps(body))
        return body

    def desired(self, etag: str = "") -> tuple[int, dict[str, Any]]:
        headers = {}
        if etag:
            headers["If-None-Match"] = etag
        try:
            return self._request("GET", "/agent/v1/desired", headers=headers)
        except AgentAPIError as e:
            if e.status == 304:
                return 304, {}
            raise

    def report(
        self, node_id: str, revision_id: str, stage: str, result: str, error: str = ""
    ) -> None:
        self._request(
            "POST",
            f"/agent/v1/nodes/{node_id}/reports",
            {
                "revision_id": revision_id,
                "stage": stage,
                "result": result,
                "error": error,
            },
        )

    def cleanup(
        self, node_id: str, assignment_id: str, result: str, error: str = ""
    ) -> None:
        self._request(
            "POST",
            f"/agent/v1/nodes/{node_id}/cleanup",
            {
                "assignment_id": assignment_id,
                "result": result,
                "error": error,
            },
        )

    def heartbeat(self, node_id: str) -> dict[str, Any]:
        """Report liveness; returns the body, which may carry the
        server-side poll interval."""
        status, body = self._request("POST", f"/agent/v1/nodes/{node_id}/heartbeat", {})
        if status != 200:
            raise AgentAPIError(status, json.dumps(body))
        return body
