#!/usr/bin/env bash
# Stop the local development stack started by scripts/dev-up.sh.
# Add --volumes to also wipe the PostgreSQL dev data.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

pkill -f "autosecrets-core" >/dev/null 2>&1 || true
pkill -f "vite.*5199" >/dev/null 2>&1 || true
pkill -f "devproxy" >/dev/null 2>&1 || true
docker compose -f deploy/compose.dev.yaml down "$@"
echo "dev stack stopped"
