#!/usr/bin/env bash
# Local stack via deploy/compose.dev.yaml.
# One address: http://127.0.0.1:18008  (console + /api + /agent)
#
# Usage: scripts/dev-up.sh [--lan]
#   --lan  also publish the Agent mTLS proxy on :18443 with an IP SAN
#          so other machines can install an Agent.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

COMPOSE=(docker compose -f deploy/compose.dev.yaml)

LAN=0
if [ "${1:-}" = "--lan" ]; then LAN=1; fi

# Drop leftover host processes from the previous (non-compose) dev-up.
pkill -f "autosecrets-core" >/dev/null 2>&1 || true
pkill -f "vite.*18008" >/dev/null 2>&1 || true
pkill -f "vite.*5199" >/dev/null 2>&1 || true
fuser -k 18443/tcp >/dev/null 2>&1 || true

if [ "$LAN" = "1" ]; then
	LAN_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
	case "$LAN_IP" in
	127.* | "")
		echo "cannot determine a LAN address (hostname -I)" >&2
		exit 1
		;;
	esac
	export AGENT_PROXY_SAN_IPS="$LAN_IP"
	export CORE_PUBLIC_AGENT_URL="https://${LAN_IP}:18443"
	COMPOSE+=(-f deploy/compose.dev.lan.yaml)
	echo "==> LAN mode: Agent API will be https://${LAN_IP}:18443"
fi

echo "==> Stack (compose.dev.yaml)"
"${COMPOSE[@]}" up -d --build

echo "==> Waiting for http://127.0.0.1:18008"
# Host HTTP_PROXY (Clash) would send 127.0.0.1 through the proxy.
for _ in $(seq 1 60); do
	wget -q -O /dev/null --no-proxy http://127.0.0.1:18008/api/v1/health 2>/dev/null && break
	sleep 1
done
wget -q -O /dev/null --no-proxy http://127.0.0.1:18008/api/v1/health 2>/dev/null || {
	echo "stack failed to become ready" >&2
	"${COMPOSE[@]}" logs --tail=40 core web >&2
	exit 1
}

CODE="$("${COMPOSE[@]}" logs core 2>/dev/null |
	grep -o 'BOOTSTRAP CODE: [^ ]*' | head -1 | awk '{print $3}' || true)"

echo
echo "=============================================="
echo "  Open:       http://127.0.0.1:18008"
if [ -n "${CODE:-}" ]; then
	echo "  Bootstrap:  $CODE   <- paste into the panel on first open"
else
	echo "  Bootstrap:  already bootstrapped; log in with the existing admin"
fi
if [ "$LAN" = "1" ]; then
	echo
	echo "  LAN mode: add the server in Nodes, then Generate connection."
	echo "  The command already includes curl -k (self-signed dev certificate)."
	echo "  Agent endpoint: https://${LAN_IP}:18443/agent/v1/install.sh"
fi
echo "  Stop:       scripts/dev-down.sh"
echo "=============================================="
