"""Unit tests for the Watermelon registry availability gate.

The gate distinguishes a real component manifest (JSON) from an SPA HTML
fallback — the failure mode observed when the adoption decision was made.
Tests use a local HTTP server so they never depend on the network.
"""

from __future__ import annotations

import http.server
import json
import os
import sys
import threading
import unittest

sys.path.insert(0, os.path.dirname(__file__))
from watermelon_registry_check import check_registry


class FakeRegistry(http.server.BaseHTTPRequestHandler):
    mode = "json"

    def do_GET(self):  # noqa: N802
        if self.mode == "json":
            body = json.dumps({
                "name": "button",
                "type": "registry:ui",
                "files": [{"path": "button.tsx", "type": "registry:ui"}],
            }).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        elif self.mode == "html":
            body = b"<!doctype html><html><body>SPA fallback</body></html>"
            self.send_response(200)
            self.send_header("Content-Type", "text/html")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        else:
            self.send_response(500)
            self.end_headers()

    def log_message(self, format, *args):  # noqa: A002 - silence
        pass


def serve(mode: str):
    FakeRegistry.mode = mode
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), FakeRegistry)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server


class RegistryCheckTest(unittest.TestCase):
    def test_accepts_real_json_manifest(self):
        server = serve("json")
        try:
            url = f"http://127.0.0.1:{server.server_address[1]}/r/index.json"
            self.assertTrue(check_registry(url))
        finally:
            server.shutdown()

    def test_rejects_spa_html_fallback(self):
        server = serve("html")
        try:
            url = f"http://127.0.0.1:{server.server_address[1]}/r/index.json"
            self.assertFalse(check_registry(url))
        finally:
            server.shutdown()

    def test_rejects_server_error(self):
        server = serve("error")
        try:
            url = f"http://127.0.0.1:{server.server_address[1]}/r/index.json"
            self.assertFalse(check_registry(url))
        finally:
            server.shutdown()

    def test_rejects_unreachable(self):
        self.assertFalse(check_registry("http://127.0.0.1:1/r/index.json"))


if __name__ == "__main__":
    unittest.main()
