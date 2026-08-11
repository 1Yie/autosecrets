#!/usr/bin/env bash
# Runs the slice-1 E2E: Core + devproxy + vite + Playwright with a node fixture.
set -euo pipefail
# Localhost traffic must never go through the environment's HTTP proxy.
export no_proxy="127.0.0.1,localhost" NO_PROXY="127.0.0.1,localhost"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'fuser -k 5199/tcp 2>/dev/null || true; rm -rf "$WORK"' EXIT

# Free the ports used by the stack (stale processes from interrupted runs).
fuser -k 5199/tcp 2>/dev/null || true
fuser -k 18080/tcp 2>/dev/null || true
fuser -k 18443/tcp 2>/dev/null || true
sleep 1

export PATH="/home/ichiyo/sdk/go1.26.5/bin:$PATH:${PATH:-}"
export GOSUMDB=off GOPROXY=https://proxy.golang.org,direct
PG_PORT="${AUTOSECRETS_TEST_PG_PORT:-55433}"

docker exec autosecrets-test-pg psql -U autosecrets -d postgres -c "DROP DATABASE IF EXISTS autosecrets_e2e WITH (FORCE)" >/dev/null
docker exec autosecrets-test-pg psql -U autosecrets -d postgres -c "CREATE DATABASE autosecrets_e2e" >/dev/null

mkdir -p "$WORK/keys" "$WORK/artifacts"
(cd "$ROOT/core" && go build -o autosecrets-core ./cmd/autosecrets-core)

CORE_LISTEN_ADDR=127.0.0.1:18080 \
CORE_KEYS_DIR="$WORK/keys" \
CORE_DB_DSN="postgres://autosecrets:test@localhost:$PG_PORT/autosecrets_e2e" \
CORE_TRUSTED_PROXY_CIDRS=127.0.0.0/8 \
CORE_PUBLIC_AGENT_URL="https://localhost:18443" \
CORE_ARTIFACT_DIR="$WORK/artifacts" \
"$ROOT/core/autosecrets-core" > "$WORK/core.log" 2>&1 &

for i in $(seq 1 30); do
  wget -q -O /dev/null http://127.0.0.1:18080/api/v1/health 2>/dev/null && break
  sleep 0.5
done
wget -q -O /dev/null http://127.0.0.1:18080/api/v1/health 2>/dev/null || {
  echo "core failed to start" >&2; cat "$WORK/core.log" >&2; exit 1; }
CODE="$(grep -o 'BOOTSTRAP CODE: [^ ]*' "$WORK/core.log" | head -1 | awk '{print $3}')"
[ -n "$CODE" ] || { echo "no bootstrap code" >&2; cat "$WORK/core.log" >&2; exit 1; }

"$ROOT/scripts/build-agent-artifact.sh" "$WORK/keys/core-signing.key" "$WORK/artifacts" >/dev/null

"$ROOT/agent/.venv/bin/python" - "$ROOT/agent" "$WORK" <<'PY' &
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]) / "tests"))
from devproxy import DevProxy
work = Path(sys.argv[2])
proxy = DevProxy("127.0.0.1:18080", work / "keys/agent-ca.crt", work / "keys/agent-ca.key", port=18443)
proxy.start()
import time
while True:
    time.sleep(10)
PY
DEVPROXY_PID=$!

(cd "$ROOT/web" && CORE_URL=http://127.0.0.1:18080 npx vite --port 5199 --strictPort --host 127.0.0.1 > "$WORK/vite.log" 2>&1) &
VITE_PID=$!
for i in $(seq 1 40); do
  wget -q -O /dev/null http://127.0.0.1:5199/ 2>/dev/null && break
  sleep 0.5
done
wget -q -O /dev/null http://127.0.0.1:5199/ 2>/dev/null || {
  echo "vite failed to start" >&2; cat "$WORK/vite.log" >&2; exit 1; }

cd "$ROOT/web"
E2E_KEYS="$WORK/keys" E2E_BOOTSTRAP_CODE="$CODE" E2E_PROXY_URL="https://localhost:18443" \
  CORE_URL="http://127.0.0.1:18080" \
  npx playwright test -c e2e/playwright.config.ts
