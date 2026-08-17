#!/usr/bin/env bash
# Stop the local development stack started by scripts/dev-up.sh.
# Add --volumes to also wipe the PostgreSQL dev data.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# The LAN proxy is an inline python process, so it has no searchable name.
# Stop it before Core so leftover Agents do not hit a dead upstream.
fuser -k 18443/tcp >/dev/null 2>&1 || true
pkill -f "autosecrets-core" >/dev/null 2>&1 || true
pkill -f "vite.*5199" >/dev/null 2>&1 || true
docker compose -f deploy/compose.dev.yaml down "$@"
echo "dev stack stopped"
