#!/usr/bin/env bash
# One-command local development stack (way B):
#   PostgreSQL via docker compose (deploy/compose.dev.yaml, port 55434)
#   Core on 127.0.0.1:18080, Web dev server on http://127.0.0.1:5199
# Logs land in .dev/; stop everything with scripts/dev-down.sh.
#
# Usage: scripts/dev-up.sh [--lan]
#   --lan  also expose the Agent API on the LAN (devproxy on :18443 with an
#          IP-SAN certificate, signed Agent artifact, CORE_PUBLIC_AGENT_URL
#          set) so other machines can install the Agent and sync Secrets.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
DEV="$ROOT/.dev"
mkdir -p "$DEV/keys" "$DEV/artifacts"

LAN=0
if [ "${1:-}" = "--lan" ]; then LAN=1; fi

LAN_IP=""
PUBLIC_AGENT_URL="${CORE_PUBLIC_AGENT_URL:-}"
INSTALL_CURL_OPTS="${CORE_INSTALL_CURL_OPTS:-}"
if [ "$LAN" = "1" ]; then
	LAN_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
	case "$LAN_IP" in
	127.* | "")
		echo "cannot determine a LAN address (hostname -I)" >&2
		exit 1
		;;
	esac
	PUBLIC_AGENT_URL="https://${LAN_IP}:18443"
	INSTALL_CURL_OPTS="AUTOSECRETS_CURL_OPTS='-k'"
	echo "==> LAN mode: Agent API will be https://${LAN_IP}:18443"
fi

# Locate the Go toolchain (not on PATH on this machine).
if [ -z "${GO:-}" ]; then
	for c in go /home/ichiyo/sdk/go1.26.5/bin/go /usr/local/go/bin/go; do
		if command -v "$c" >/dev/null 2>&1 || [ -x "$c" ]; then
			GO="$c"
			break
		fi
	done
fi
[ -n "${GO:-}" ] || {
	echo "go toolchain not found (set GO=...)" >&2
	exit 1
}
export PATH="$(dirname "$GO"):$PATH"

# Drop a leftover LAN proxy before postgres comes up. Otherwise a previous
# --lan session keeps accepting Agent traffic while Core is still down.
if [ "$LAN" = "1" ]; then
	fuser -k 18443/tcp >/dev/null 2>&1 || true
fi

echo "==> PostgreSQL (compose)"
docker compose -f deploy/compose.dev.yaml up -d postgres
for _ in $(seq 1 30); do
	[ "$(docker inspect -f '{{.State.Health.Status}}' autosecrets-dev-postgres-1 2>/dev/null)" = "healthy" ] && break
	sleep 1
done
docker inspect -f '{{.State.Health.Status}}' autosecrets-dev-postgres-1 2>/dev/null | grep -q healthy ||
	{
		echo "postgres did not become healthy" >&2
		exit 1
	}

echo "==> Core (127.0.0.1:18080)"
# A stale Core process would hold :18080 and answer with a dead pool.
# The leftover LAN proxy is already gone, so Agents cannot hit a dead Core.
pkill -f "autosecrets-core" >/dev/null 2>&1 || true
(cd core && "$GO" build -o ../.dev/autosecrets-core ./cmd/autosecrets-core)

CORE_LISTEN_ADDR=127.0.0.1:18080 \
	CORE_KEYS_DIR="$DEV/keys" \
	CORE_DB_DSN="postgres://autosecrets:test@localhost:55434/autosecrets" \
	CORE_TRUSTED_PROXY_CIDRS=127.0.0.0/8 \
	CORE_PUBLIC_AGENT_URL="$PUBLIC_AGENT_URL" \
	CORE_ARTIFACT_DIR="$DEV/artifacts" \
	CORE_INSTALL_CURL_OPTS="$INSTALL_CURL_OPTS" \
	nohup "$DEV/autosecrets-core" >"$DEV/core.log" 2>&1 &
CORE_PID=$!
for _ in $(seq 1 30); do
	wget -q -O /dev/null http://127.0.0.1:18080/api/v1/health 2>/dev/null && break
	sleep 0.5
done
wget -q -O /dev/null http://127.0.0.1:18080/api/v1/health 2>/dev/null || {
	echo "core failed to start; see .dev/core.log" >&2
	tail -5 "$DEV/core.log" >&2
	exit 1
}
echo "   core pid $CORE_PID (log: .dev/core.log)"

if [ "$LAN" = "1" ]; then
	echo "==> Signed Agent artifact"
	scripts/build-agent-artifact.sh "$DEV/keys/core-signing.key" "$DEV/artifacts"
	echo "==> Devproxy (mTLS Agent endpoint on 0.0.0.0:18443)"
	# Inline-python processes do not carry a searchable name; free the port.
	fuser -k 18443/tcp >/dev/null 2>&1 || true
	sleep 1
	"$ROOT/agent/.venv/bin/python" - "$ROOT/agent" "$DEV" "$LAN_IP" <<'PY' &
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]) / "tests"))
from devproxy import DevProxy
work = Path(sys.argv[2])
lan_ip = sys.argv[3]
proxy = DevProxy("127.0.0.1:18080", work / "keys/agent-ca.crt", work / "keys/agent-ca.key",
                 port=18443, bind="0.0.0.0", hostname=lan_ip, san_ips=(lan_ip,))
proxy.start()
import time
while True:
    time.sleep(10)
PY
	sleep 1
	echo "   devproxy pid $!"
fi

CODE="$(grep -o 'BOOTSTRAP CODE: [^ ]*' "$DEV/core.log" 2>/dev/null | head -1 | awk '{print $3}' || true)"

echo "==> Web dev server (http://127.0.0.1:5199)"
# --strictPort refuses to start twice; clear any stale dev server first.
pkill -f "vite.*5199" >/dev/null 2>&1 || true
(cd web && CORE_URL=http://127.0.0.1:18080 nohup bun run dev --port 5199 --strictPort --host 127.0.0.1 \
	>"$DEV/vite.log" 2>&1 &)
for _ in $(seq 1 40); do
	wget -q -O /dev/null http://127.0.0.1:5199/ 2>/dev/null && break
	sleep 0.5
done
wget -q -O /dev/null http://127.0.0.1:5199/ 2>/dev/null || {
	echo "vite failed to start; see .dev/vite.log" >&2
	tail -5 "$DEV/vite.log" >&2
	exit 1
}

echo
echo "=============================================="
echo "  Web:        http://127.0.0.1:5199"
echo "  Core:       http://127.0.0.1:18080 (health: /api/v1/health)"
if [ -n "${CODE:-}" ]; then
	echo "  Bootstrap:  $CODE   <- paste into the panel on first open"
else
	echo "  Bootstrap:  already bootstrapped; log in with the existing admin"
fi
if [ "$LAN" = "1" ]; then
	echo
	echo "  LAN mode: generate the Install Command in the Web UI (Nodes ->"
	echo "  Add server), then on the target machine run it with TLS check"
	echo "  skipped (self-signed dev certificate):"
	echo "    AUTOSECRETS_CURL_OPTS='-k' <the generated curl command>"
	echo "  Agent endpoint: https://${LAN_IP}:18443/agent/v1/install.sh"
fi
echo "  Stop:       scripts/dev-down.sh"
echo "=============================================="
