#!/usr/bin/env sh
# Phase-0 risk gate: cross-language envelope interoperability.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if [ -x "$HOME/.local/go/bin/go" ]; then
	export PATH="$HOME/.local/go/bin:$PATH"
fi

echo "== Go envelope tests (vectors + live Python round trip) =="
(cd "$ROOT/core" && go test ./internal/envelope/ -run 'TestVectors|TestPythonInterop' -v)

echo "== Python envelope vector tests =="
(cd "$ROOT/agent" && .venv/bin/python -m pytest tests/test_vectors.py -q)

echo "interop-verify: OK"
