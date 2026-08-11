"""Test-only TLS reverse proxy that mirrors the Caddy contract: it terminates
mTLS, verifies the client certificate against the Agent CA, and forwards the
certificate serial to Core in X-Autosecrets-Client-Cert. Core only trusts the
header from its configured proxy CIDRs, so this proxy is the trust boundary in
tests, exactly as Caddy is in production (ADR-0010).
"""

from __future__ import annotations

import http.client
import json
import ssl
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

SERIAL_HEADER = "X-Autosecrets-Client-Cert"
FORWARD_HEADERS = ("Content-Type", "X-Correlation-ID")


def build_server_context(ca_cert: Path, ca_key: Path, hostname: str = "localhost") -> ssl.SSLContext:
    import datetime

    from cryptography import x509
    from cryptography.hazmat.primitives import hashes, serialization
    from cryptography.hazmat.primitives.asymmetric import rsa
    from cryptography.x509.oid import NameOID

    ca = x509.load_pem_x509_certificate(ca_cert.read_bytes())
    ca_key_obj = serialization.load_pem_private_key(ca_key.read_bytes(), password=None)
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, hostname)])
    now = datetime.datetime.now(datetime.UTC)
    cert = (
        x509.CertificateBuilder()
        .subject_name(name)
        .issuer_name(ca.subject)
        .public_key(key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now - datetime.timedelta(minutes=5))
        .not_valid_after(now + datetime.timedelta(days=1))
        .add_extension(x509.SubjectAlternativeName([x509.DNSName(hostname)]), critical=False)
        .add_extension(x509.AuthorityKeyIdentifier.from_issuer_public_key(ca_key_obj.public_key()), critical=False)
        .sign(ca_key_obj, hashes.SHA256())
    )
    ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    import tempfile
    cert_file = tempfile.NamedTemporaryFile("w", suffix=".pem", delete=False)
    cert_file.write(cert.public_bytes(serialization.Encoding.PEM).decode())
    cert_file.close()
    key_file = tempfile.NamedTemporaryFile("w", suffix=".pem", delete=False)
    key_file.write(key.private_bytes(serialization.Encoding.PEM,
                                     serialization.PrivateFormat.PKCS8,
                                     serialization.NoEncryption()).decode())
    key_file.close()
    ctx.load_cert_chain(certfile=cert_file.name, keyfile=key_file.name)
    ctx.load_verify_locations(cafile=str(ca_cert))
    # Client certificates are verified manually per-path (pre-certificate
    # routes must work without one), mirroring the Caddy handle blocks.
    ctx.verify_mode = ssl.CERT_OPTIONAL
    ctx.ca_cert_file = str(ca_cert)
    return ctx


def make_proxy_handler(upstream: str, ca_cert_file: str) -> type[BaseHTTPRequestHandler]:
    class ProxyHandler(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        PRE_CERT_PATHS = ("/agent/v1/enroll", "/agent/v1/install.sh", "/agent/v1/ca.pem")

        def _forward(self) -> None:
            serial = None
            if self.connection is not None:
                der = self.connection.getpeercert(binary_form=True)
                if der:
                    from cryptography import x509
                    cert = x509.load_der_x509_certificate(der)
                    ca = x509.load_pem_x509_certificate(Path(ca_cert_file).read_bytes())
                    try:
                        cert.verify_directly_issued_by(ca)
                    except Exception:
                        self.send_error(403, "client certificate not issued by the Agent CA")
                        return
                    serial = f"{cert.serial_number:x}"
            pre_cert = self.path.startswith(self.PRE_CERT_PATHS) or "/agent/v1/artifacts/" in self.path
            if not serial and not pre_cert:
                self.send_error(403, "client certificate required")
                return
            length = int(self.headers.get("Content-Length", 0))
            body = self.rfile.read(length) if length else None
            host, port = upstream.rsplit(":", 1)
            conn = http.client.HTTPConnection(host, int(port), timeout=30)
            headers = {k: v for k, v in self.headers.items() if k in FORWARD_HEADERS}
            if serial:
                headers[SERIAL_HEADER] = serial
            conn.request(self.command, self.path, body=body, headers=headers)
            resp = conn.getresponse()
            data = resp.read()
            self.send_response(resp.status)
            for k, v in resp.getheaders():
                if k.lower() in ("content-type", "content-length", "etag"):
                    self.send_header(k, v)
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
            conn.close()

        do_GET = do_POST = do_PUT = do_DELETE = _forward

        def log_message(self, fmt, *args):  # silence
            pass

    return ProxyHandler


class DevProxy:
    def __init__(self, upstream: str, ca_cert: Path, ca_key: Path, port: int = 0):
        ctx = build_server_context(ca_cert, ca_key)
        self.httpd = ThreadingHTTPServer(("127.0.0.1", port),
                                        make_proxy_handler(upstream, str(ca_cert)))
        self.httpd.socket = ctx.wrap_socket(self.httpd.socket, server_side=True)
        self.port = self.httpd.server_address[1]
        self._thread = threading.Thread(target=self.httpd.serve_forever, daemon=True)

    def start(self) -> None:
        self._thread.start()

    def stop(self) -> None:
        self.httpd.shutdown()

    @property
    def url(self) -> str:
        return f"https://localhost:{self.port}"


def http_json(method: str, url: str, body: dict | None = None, cookies: str = "",
              headers: dict | None = None) -> tuple[int, dict, dict[str, str]]:
    import urllib.request
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method, headers={
        "Content-Type": "application/json" if data else "",
        "Cookie": cookies,
        **(headers or {}),
    })
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            raw = resp.read().decode()
            out_headers = dict(resp.headers.items())
            return resp.status, (json.loads(raw) if raw else {}), out_headers
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        try:
            return e.code, json.loads(raw), dict(e.headers.items())
        except json.JSONDecodeError:
            return e.code, {}, dict(e.headers.items())
