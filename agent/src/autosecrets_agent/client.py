"""HTTP client for the Agent API: plain HTTPS for enrollment, mTLS after."""

from __future__ import annotations

import json
import ssl
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
    def __init__(self, server_url: str, ca_bundle: str = "",
                 cert_pem: str = "", key_pem: str = ""):
        self.base = server_url.rstrip("/")
        if ca_bundle and Path(ca_bundle).exists():
            ctx = ssl.create_default_context(cafile=ca_bundle)
        else:
            # No bundle yet (pre-enrollment) or system trust: default context.
            ctx = ssl.create_default_context()
        if cert_pem and key_pem:
            # Client certificate for mTLS (the proxy terminates TLS and
            # forwards the serial to Core; the Agent never sets that header).
            import tempfile
            with tempfile.NamedTemporaryFile("w", suffix=".pem", delete=False) as c, \
                 tempfile.NamedTemporaryFile("w", suffix=".pem", delete=False) as k:
                c.write(cert_pem)
                k.write(key_pem)
                self._cert_file, self._key_file = c.name, k.name
            ctx.load_cert_chain(self._cert_file, self._key_file)
        self.ctx = ctx

    def close(self) -> None:
        for attr in ("_cert_file", "_key_file"):
            if hasattr(self, attr):
                Path(getattr(self, attr)).unlink(missing_ok=True)

    def _request(self, method: str, path: str, body: Any | None = None,
                 headers: dict[str, str] | None = None) -> tuple[int, dict[str, Any]]:
        data = None
        if body is not None:
            data = json.dumps(body).encode()
            headers = {**(headers or {}), "Content-Type": "application/json"}
        req = urllib.request.Request(self.base + path, data=data, method=method,
                                     headers=headers or {})
        try:
            with urllib.request.urlopen(req, context=self.ctx, timeout=30) as resp:
                raw = resp.read()
                return resp.status, (json.loads(raw) if raw else {})
        except urllib.error.HTTPError as e:
            raw = e.read().decode("utf-8", "replace")
            raise AgentAPIError(e.code, raw) from None

    def enroll(self, token: str, name: str, age_pubkey: str, csr: str) -> dict[str, Any]:
        status, body = self._request("POST", "/agent/v1/enroll", {
            "token": token, "name": name,
            "age_pubkey": age_pubkey, "csr": csr,
        })
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

    def report(self, node_id: str, revision_id: str, stage: str, result: str,
               error: str = "") -> None:
        self._request("POST", f"/agent/v1/nodes/{node_id}/reports", {
            "revision_id": revision_id, "stage": stage, "result": result, "error": error,
        })

    def cleanup(self, node_id: str, assignment_id: str, result: str,
                error: str = "") -> None:
        self._request("POST", f"/agent/v1/nodes/{node_id}/cleanup", {
            "assignment_id": assignment_id, "result": result, "error": error,
        })

    def heartbeat(self, node_id: str) -> None:
        self._request("POST", f"/agent/v1/nodes/{node_id}/heartbeat", {})
