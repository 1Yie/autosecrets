#!/usr/bin/env bash
# Stop the local development stack started by scripts/dev-up.sh.
# Add --volumes to also wipe PostgreSQL, keys, and Agent identity.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# Leftover host processes from the previous (non-compose) dev-up.
fuser -k 18443/tcp >/dev/null 2>&1 || true
pkill -f "autosecrets-core" >/dev/null 2>&1 || true
pkill -f "vite.*18008" >/dev/null 2>&1 || true
pkill -f "vite.*5199" >/dev/null 2>&1 || true

docker compose -f deploy/compose.dev.yaml \
	-f deploy/compose.dev.lan.yaml down "$@" 2>/dev/null ||
	docker compose -f deploy/compose.dev.yaml down "$@"
echo "dev stack stopped"
