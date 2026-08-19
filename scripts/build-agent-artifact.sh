#!/usr/bin/env bash
# Repo wrapper around agent/docker/build-artifacts.sh.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export AGENT_SRC="${AGENT_SRC:-$ROOT/agent}"
exec "$ROOT/agent/docker/build-artifacts.sh" "$@"
