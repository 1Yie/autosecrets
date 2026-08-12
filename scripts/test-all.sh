#!/usr/bin/env bash
# Unified test entry point: one command runs the whole suite the way CI runs
# it. Starts (or reuses) the shared PostgreSQL container, then runs Core,
# Agent, and Web tests with the same environment.
#
# Usage:
#   scripts/test-all.sh            # full suite
#   TEST_DATABASE_URL=... scripts/test-all.sh   # reuse an existing PostgreSQL
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [ -z "${TEST_DATABASE_URL:-}" ]; then
  echo "==> Starting shared PostgreSQL container (autosecrets-test-pg)"
  docker rm -f autosecrets-test-pg >/dev/null 2>&1 || true
  docker run -d --name autosecrets-test-pg \
    -e POSTGRES_DB=autosecrets -e POSTGRES_USER=autosecrets \
    -e POSTGRES_PASSWORD=test -p 55433:5432 postgres:17-alpine >/dev/null
  for _ in $(seq 1 30); do
    docker exec autosecrets-test-pg pg_isready -U autosecrets >/dev/null 2>&1 && break
    sleep 1
  done
  docker exec autosecrets-test-pg pg_isready -U autosecrets >/dev/null 2>&1 \
    || { echo "PostgreSQL did not become ready" >&2; exit 1; }
  export TEST_DATABASE_URL="postgres://autosecrets:test@localhost:55433/autosecrets"
fi

echo "==> Core: fmt, vet, tests, coverage gate (70%)"
GO=${GO:-go}
(cd core && "$GO" fmt ./... >/dev/null && "$GO" vet ./... \
  && "$GO" test ./... -count=1 -coverprofile=coverage.out)
python3 scripts/coverage_gate.py core/coverage.out 70
rm -f core/coverage.out

echo "==> Agent: pytest (envelope unit + integration against the devproxy)"
# A stale Core process from a previous run would hold :18080 and answer with
# a dead database connection; the Agent suite must start clean.
pkill -f "autosecrets-core" >/dev/null 2>&1 || true
(cd agent && .venv/bin/python -m pytest tests/ -q)

echo "==> Web: lint, unit tests, build"
(cd web && bun run lint && bun run test && bun run build)

echo "==> All suites passed"
