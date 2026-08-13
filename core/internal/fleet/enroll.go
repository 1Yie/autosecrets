package fleet

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/database"
	"autosecrets.dev/core/internal/middleware"
	"github.com/google/uuid"
)

// handleInstallCommand issues a ten-minute, single-use Enrollment Token and
// renders the Install Command. The Token appears in this response only.
func (h *Handler) handleInstallCommand(w http.ResponseWriter, r *http.Request) {
	if h.cfg.PublicAgentURL == "" {
		middleware.WriteError(w, http.StatusServiceUnavailable, "unavailable",
			"CORE_PUBLIC_AGENT_URL is not configured")
		return
	}
	var body struct {
		Name      string `json:"name"`
		BundleDir string `json:"bundle_dir"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !middleware.ValidName(body.Name, 64) {
		body.Name = "node-" + uuid.NewString()[:8]
	}
	if body.BundleDir != "" && !strings.HasPrefix(body.BundleDir, "~") &&
		!strings.HasPrefix(body.BundleDir, "/") {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request",
			"bundle_dir must be an absolute path or start with ~/")
		return
	}
	token, err := crypto.NewSecret(192)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	expiresAt := h.now().Add(tokenTTL)
	if err := h.store.CreateEnrollmentToken(r.Context(), crypto.HashToken(token), strings.TrimSpace(body.Name), expiresAt); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	base := strings.TrimSuffix(h.cfg.PublicAgentURL, "/")
	curl := "curl -fsSL"
	extra := ""
	if h.cfg.InstallCurlOpts != "" {
		curl = "curl -k -fsSL"
		extra = " --insecure"
	}
	if body.BundleDir != "" {
		extra += fmt.Sprintf(" --bundle-dir %q", body.BundleDir)
	}
	command := fmt.Sprintf(
		"%s %s%s/install.sh | sudo bash -s -- --server %s --token %s --name %q%s",
		curl, base, h.agentBase, base, token, strings.TrimSpace(body.Name), extra)
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "token.issue", Resource: "",
		Result:        "expires=" + expiresAt.UTC().Format(time.RFC3339),
		CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusCreated, map[string]string{
		"command": command, "expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

// handleInstallScript serves the public install script with the Core signing
// public key embedded. The Enrollment Token is never embedded; it is passed
// as an argument when the command runs.
func (h *Handler) handleInstallScript(w http.ResponseWriter, r *http.Request) {
	if h.cfg.PublicAgentURL == "" {
		middleware.WriteError(w, http.StatusServiceUnavailable, "unavailable",
			"CORE_PUBLIC_AGENT_URL is not configured")
		return
	}
	pubPEM, err := h.signer.PublicKeyPEM()
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	script := fmt.Sprintf(installScriptTemplate, h.agentBase, h.agentBase, string(pubPEM))
	w.Header().Set("Content-Type", "text/x-shellscript")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(script))
}

// handleCAPEM serves the internal Agent CA certificate.
func (h *Handler) handleCAPEM(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(h.ca.CertPEM())
}

// handleArtifact serves signed Agent artifacts.
func (h *Handler) handleArtifact(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !strings.HasPrefix(name, "autosecrets-agent-linux-") ||
		(!strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".tar.gz.sig")) {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "artifact not found")
		return
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "artifact not found")
		return
	}
	path := filepath.Join(h.cfg.ArtifactDir, name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "artifact not found")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, path)
}

// handleEnroll is the only pre-certificate Agent route. It consumes the
// one-time Token, registers the node with its public keys, and issues a
// short-lived client certificate from the internal CA.
func (h *Handler) handleEnroll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token     string `json:"token"`
		Name      string `json:"name"`
		AgePubkey string `json:"age_pubkey"`
		CSR       string `json:"csr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
		body.Token == "" || body.AgePubkey == "" || body.CSR == "" {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "token, age_pubkey, and csr are required")
		return
	}
	tokenRow, err := h.store.ConsumeEnrollmentToken(r.Context(), crypto.HashToken(body.Token), h.now())
	if err != nil {
		middleware.WriteError(w, http.StatusForbidden, "forbidden", "invalid, expired, or already-used token")
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
	certPEM, serial, expiresAt, err := h.ca.IssueAgentCert(nodeID, []byte(body.CSR), certTTL)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "invalid CSR")
		return
	}
	if err := h.store.RegisterNode(r.Context(), nodeID, name, serial, body.AgePubkey, string(certPEM), expiresAt); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: "agent:" + name, Action: "node.enroll", Resource: nodeID,
		Result: "ok", CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusCreated, map[string]string{
		"node_id":    nodeID,
		"cert_pem":   string(certPEM),
		"ca_pem":     string(h.ca.CertPEM()),
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

const installScriptTemplate = `#!/bin/sh
# AutoSecrets Agent installer (ADR-0013).
set -eu

SERVER=""
TOKEN=""
NODE_NAME=""
INSECURE=""
BUNDLE_DIR=""
while [ $# -gt 0 ]; do
  case "$1" in
    --server) SERVER="$2"; shift 2 ;;
    --token) TOKEN="$2"; shift 2 ;;
    --name) NODE_NAME="$2"; shift 2 ;;
    --bundle-dir) BUNDLE_DIR="$2"; shift 2 ;;
    --insecure) INSECURE="1"; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done
[ -n "$SERVER" ] && [ -n "$TOKEN" ] || { echo "usage: install.sh --server URL --token TOKEN" >&2; exit 2; }

PREFIX="${AUTOSECRETS_PREFIX:-/opt/autosecrets-agent}"
CONFIG_DIR="${AUTOSECRETS_CONFIG_DIR:-/etc/autosecrets-agent}"
STATE_DIR="${AUTOSECRETS_STATE_DIR:-/var/lib/autosecrets-agent}"

if [ -z "$BUNDLE_DIR" ]; then
  BUNDLE_DIR="~/.autosecrets"
fi
USER_HOME="$HOME"
if [ -n "${SUDO_USER:-}" ] && [ "$SUDO_USER" != "root" ]; then
  USER_HOME="$(getent passwd "$SUDO_USER" | cut -d: -f6)"
fi
case "$BUNDLE_DIR" in
  "~/"*) BUNDLE_DIR="$USER_HOME/${BUNDLE_DIR#\~/}" ;;
esac

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

if [ -n "$INSECURE" ]; then
  CURL="curl -fsSL -k"
else
  CURL="curl -fsSL ${AUTOSECRETS_CURL_OPTS:-}"
fi
$CURL "$SERVER%[1]s/artifacts/autosecrets-agent-linux-$ART_ARCH.tar.gz" -o "$TMP/agent.tar.gz"
$CURL "$SERVER%[2]s/artifacts/autosecrets-agent-linux-$ART_ARCH.tar.gz.sig" -o "$TMP/agent.tar.gz.sig"
verify_artifact "$TMP/agent.tar.gz" "$TMP/agent.tar.gz.sig"

echo "==> Installing to $PREFIX"
mkdir -p "$PREFIX" "$CONFIG_DIR" "$STATE_DIR/identity" "$STATE_DIR/bundles"
tar -xzf "$TMP/agent.tar.gz" -C "$PREFIX"

SIGNING_KEY_B64="$(openssl pkey -pubin -in "$PUBKEY_FILE" -outform DER 2>/dev/null | tail -c 32 | base64 -w0)"
CA_BUNDLE_LINE=""
if [ -n "$INSECURE" ] || [ -n "${AUTOSECRETS_CURL_OPTS:-}" ]; then
  $CURL "$SERVER%[1]s/ca.pem" -o "$STATE_DIR/identity/ca.pem"
  CA_BUNDLE_LINE="ca_bundle = \"$STATE_DIR/identity/ca.pem\""
fi
cat > "$CONFIG_DIR/config.toml" <<EOF
server_url = "$SERVER"
identity_dir = "$STATE_DIR/identity"
bundle_dir = "$BUNDLE_DIR"
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
