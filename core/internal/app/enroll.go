package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/store"
	"github.com/google/uuid"
)

// handleInstallCommand issues a ten-minute, single-use Enrollment Token and
// renders the Install Command. The Token appears in this response only.
func (a *App) handleInstallCommand(w http.ResponseWriter, r *http.Request) {
	if a.cfg.PublicAgentURL == "" {
		writeJSON(w, http.StatusServiceUnavailable,
			map[string]string{"error": "CORE_PUBLIC_AGENT_URL is not configured"})
		return
	}
	var body struct{ Name string `json:"name"` }
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !validName(body.Name, 64) {
		body.Name = "node-" + uuid.NewString()[:8]
	}
	token, err := crypto.NewSecret(192)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	expiresAt := a.now().Add(tokenTTL)
	if err := a.store.CreateEnrollmentToken(r.Context(), crypto.HashToken(token), strings.TrimSpace(body.Name), expiresAt); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	base := strings.TrimSuffix(a.cfg.PublicAgentURL, "/")
	command := fmt.Sprintf(
		"curl -fsSL %s%s/install.sh | sudo bash -s -- --server %s --token %s --name %q",
		base, a.agentBase, base, token, strings.TrimSpace(body.Name))
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "token.issue", Resource: "",
		Result: "expires=" + expiresAt.UTC().Format(time.RFC3339),
		CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusCreated, map[string]string{
		"command": command, "expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

// handleInstallScript serves the public install script with the Core signing
// public key embedded. The Enrollment Token is never embedded; it is passed
// as an argument when the command runs.
func (a *App) handleInstallScript(w http.ResponseWriter, r *http.Request) {
	if a.cfg.PublicAgentURL == "" {
		writeJSON(w, http.StatusServiceUnavailable,
			map[string]string{"error": "CORE_PUBLIC_AGENT_URL is not configured"})
		return
	}
	pubPEM, err := a.signer.PublicKeyPEM()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	script := fmt.Sprintf(installScriptTemplate, a.agentBase, a.agentBase, string(pubPEM))
	w.Header().Set("Content-Type", "text/x-shellscript")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(script))
}

// handleCAPEM serves the internal Agent CA certificate. It is public by
// design: the installer needs it to trust the Agent TLS endpoint in dev and
// Caddy needs it mounted anyway; it signs nothing by itself.
func (a *App) handleCAPEM(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(a.ca.CertPEM())
}

const installScriptTemplate = `#!/bin/sh
# AutoSecrets Agent installer (ADR-0013: Core-hosted signed artifacts with
# embedded key). Usage: curl -fsSL <server>/agent/v1/install.sh | sudo bash -s -- --server <url> --token <token>
set -eu

SERVER=""
TOKEN=""
NODE_NAME=""
while [ $# -gt 0 ]; do
  case "$1" in
    --server) SERVER="$2"; shift 2 ;;
    --token) TOKEN="$2"; shift 2 ;;
    --name) NODE_NAME="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done
[ -n "$SERVER" ] && [ -n "$TOKEN" ] || { echo "usage: install.sh --server URL --token TOKEN" >&2; exit 2; }

PREFIX="${AUTOSECRETS_PREFIX:-/opt/autosecrets-agent}"
CONFIG_DIR="${AUTOSECRETS_CONFIG_DIR:-/etc/autosecrets-agent}"
STATE_DIR="${AUTOSECRETS_STATE_DIR:-/var/lib/autosecrets-agent}"

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ART_ARCH="amd64" ;;
  aarch64|arm64) ART_ARCH="arm64" ;;
  *) echo "unsupported architecture: $ARCH" >&2; exit 2 ;;
esac

require_openssl() {
  command -v openssl >/dev/null 2>&1 || { echo "openssl is required to verify the agent artifact" >&2; exit 2; }
  command -v python3 >/dev/null 2>&1 || { echo "python3 >= 3.11 is required to run the agent" >&2; exit 2; }
}

verify_artifact() {
  artifact="$1"; sig="$2"
  openssl pkeyutl -verify -rawin -pubin -inkey "$PUBKEY_FILE" -sigfile "$sig" -in "$artifact" >/dev/null 2>&1 \
    || { echo "signature verification FAILED; refusing to install" >&2; exit 3; }
}

echo "==> Downloading and verifying the signed agent artifact"
require_openssl
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
PUBKEY_FILE="$TMP/signing.pub"
cat > "$PUBKEY_FILE" <<'PUBKEY_EOF'
%[3]s
PUBKEY_EOF

CURL="curl -fsSL ${AUTOSECRETS_CURL_OPTS:-}"
$CURL "$SERVER%[1]s/artifacts/autosecrets-agent-linux-$ART_ARCH.tar.gz" -o "$TMP/agent.tar.gz"
$CURL "$SERVER%[2]s/artifacts/autosecrets-agent-linux-$ART_ARCH.tar.gz.sig" -o "$TMP/agent.tar.gz.sig"
verify_artifact "$TMP/agent.tar.gz" "$TMP/agent.tar.gz.sig"

echo "==> Installing to $PREFIX"
mkdir -p "$PREFIX" "$CONFIG_DIR" "$STATE_DIR/identity" "$STATE_DIR/bundles"
tar -xzf "$TMP/agent.tar.gz" -C "$PREFIX"

SIGNING_KEY_B64="$(openssl pkey -pubin -in "$PUBKEY_FILE" -outform DER 2>/dev/null | tail -c 32 | base64 -w0)"
CA_BUNDLE_LINE=""
if [ -n "${AUTOSECRETS_CURL_OPTS:-}" ]; then
  # Dev/E2E only: trust the internal Agent CA for the server TLS endpoint.
  $CURL "$SERVER%[1]s/ca.pem" -o "$STATE_DIR/identity/ca.pem"
  CA_BUNDLE_LINE="ca_bundle = \"$STATE_DIR/identity/ca.pem\""
fi
cat > "$CONFIG_DIR/config.toml" <<EOF
server_url = "$SERVER"
identity_dir = "$STATE_DIR/identity"
bundle_dir = "$STATE_DIR/bundles"
name = "${NODE_NAME:-$(hostname)}"
signing_public_key = "$SIGNING_KEY_B64"
$CA_BUNDLE_LINE
EOF

echo "==> Enrolling with the one-time token"
"$PREFIX/autosecrets-agent" enroll --server "$SERVER" --token "$TOKEN" --config "$CONFIG_DIR/config.toml"

if command -v systemctl >/dev/null 2>&1 && [ -z "${AUTOSECRETS_NO_SYSTEMD:-}" ]; then
  cat > /etc/systemd/system/autosecrets-agent.service <<EOF
[Unit]
Description=AutoSecrets Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$PREFIX/autosecrets-agent serve --config $CONFIG_DIR/config.toml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now autosecrets-agent.service
  echo "==> Agent installed and running via systemd (autosecrets-agent.service)"
else
  echo "==> First convergence pass"
  "$PREFIX/autosecrets-agent" sync --config "$CONFIG_DIR/config.toml"
  echo "==> No systemd detected; run '$PREFIX/autosecrets-agent serve --config $CONFIG_DIR/config.toml' to keep polling"
fi
`

// handleArtifact serves signed Agent artifacts. Names are restricted to the
// artifact pattern so the endpoint cannot be used for path traversal.
func (a *App) handleArtifact(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !strings.HasPrefix(name, "autosecrets-agent-linux-") ||
		(!strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".tar.gz.sig")) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "artifact not found"})
		return
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "artifact not found"})
		return
	}
	path := filepath.Join(a.cfg.ArtifactDir, name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "artifact not found"})
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, path)
}

// handleEnroll is the only pre-certificate Agent route. It consumes the
// one-time Token, registers the node with its public keys, and issues a
// short-lived client certificate from the internal CA.
func (a *App) handleEnroll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token     string `json:"token"`
		Name      string `json:"name"`
		AgePubkey string `json:"age_pubkey"`
		CSR       string `json:"csr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
		body.Token == "" || body.AgePubkey == "" || body.CSR == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token, age_pubkey, and csr are required"})
		return
	}
	tokenRow, err := a.store.ConsumeEnrollmentToken(r.Context(), crypto.HashToken(body.Token), a.now())
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid, expired, or already-used token"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = tokenRow.Name
	}
	if name == "" {
		name = "node-" + uuid.NewString()[:8]
	}
	nodeID := uuid.NewString()
	certPEM, serial, expiresAt, err := a.ca.IssueAgentCert(nodeID, []byte(body.CSR), certTTL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid CSR"})
		return
	}
	if err := a.store.RegisterNode(r.Context(), nodeID, name, serial, body.AgePubkey, string(certPEM), expiresAt); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: "agent:" + name, Action: "node.enroll", Resource: nodeID,
		Result: "ok", CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusCreated, map[string]string{
		"node_id":   nodeID,
		"cert_pem":  string(certPEM),
		"ca_pem":    string(a.ca.CertPEM()),
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}
