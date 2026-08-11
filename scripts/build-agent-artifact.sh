#!/usr/bin/env bash
# Builds the signed Agent artifact served by Core: a self-contained venv
# tarball plus an Ed25519 signature from the Core signing key (ADR-0013).
# Usage: build-agent-artifact.sh <signing-key> <out-dir>
set -euo pipefail

KEY="${1:?signing key path required}"
OUT="${2:?output dir required}"
ARCH="${3:-amd64}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

python3 -m venv "$TMP/venv"
"$TMP/venv/bin/pip" install --quiet --target "$TMP/site" "$ROOT/agent"
cat > "$TMP/autosecrets-agent" <<'LAUNCHER'
#!/bin/sh
DIR=$(dirname "$0")
export PYTHONPATH="$DIR/site"
exec python3 -m autosecrets_agent.cli "$@"
LAUNCHER
chmod +x "$TMP/autosecrets-agent"

mkdir -p "$OUT"
TARBALL="$OUT/autosecrets-agent-linux-$ARCH.tar.gz"
tar -czf "$TARBALL" -C "$TMP" autosecrets-agent site
openssl pkeyutl -sign -rawin -inkey "$KEY" -in "$TARBALL" -out "$TARBALL.sig"
echo "artifact: $TARBALL ($(stat -c%s "$TARBALL") bytes) + signature"
