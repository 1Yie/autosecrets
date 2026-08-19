#!/bin/sh
set -eu

CONFIG="${AGENT_CONFIG:-/etc/autosecrets-agent/config.toml}"
IDENTITY="${AGENT_IDENTITY_DIR:-/var/lib/autosecrets-agent/identity}"
BUNDLES="${AGENT_BUNDLE_DIR:-/var/lib/autosecrets-agent/bundles}"
CA_BUNDLE="${AGENT_CA_BUNDLE:-/keys/agent-ca.crt}"
SIGNING_KEY="${CORE_SIGNING_KEY:-/keys/core-signing.key}"
SERVER_URL="${AGENT_SERVER_URL:-https://agent-proxy:18443}"

mkdir -p "$(dirname "$CONFIG")" "$IDENTITY" "$BUNDLES"

i=0
while [ ! -f "$CA_BUNDLE" ] || [ ! -f "$SIGNING_KEY" ]; do
	i=$((i + 1))
	if [ "$i" -ge 60 ]; then
		echo "timed out waiting for Core key material" >&2
		exit 1
	fi
	sleep 1
done

SIGNING_PUBLIC="$(
	python3 - "$SIGNING_KEY" <<'PY'
import base64
import sys
from pathlib import Path

from cryptography.hazmat.primitives import serialization

key = serialization.load_pem_private_key(Path(sys.argv[1]).read_bytes(), None)
print(base64.b64encode(key.public_key().public_bytes_raw()).decode())
PY
)"

cat >"$CONFIG" <<EOF
server_url = "$SERVER_URL"
identity_dir = "$IDENTITY"
bundle_dir = "$BUNDLES"
name = "${AGENT_NAME:-compose-dev}"
signing_public_key = "$SIGNING_PUBLIC"
ca_bundle = "$CA_BUNDLE"
EOF

if [ -f "$IDENTITY/state.json" ]; then
	exec autosecrets-agent serve --config "$CONFIG"
fi

if [ -n "${AGENT_ENROLL_TOKEN:-}" ]; then
	autosecrets-agent enroll --config "$CONFIG" --token "$AGENT_ENROLL_TOKEN" \
		--server "$SERVER_URL"
	exec autosecrets-agent serve --config "$CONFIG"
fi

echo "agent waiting for enrollment (set AGENT_ENROLL_TOKEN or write $IDENTITY/state.json)"
while true; do
	if [ -f "$IDENTITY/state.json" ]; then
		exec autosecrets-agent serve --config "$CONFIG"
	fi
	sleep 5
done
