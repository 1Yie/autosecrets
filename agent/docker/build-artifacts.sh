#!/usr/bin/env bash
# Build and sign Agent artifacts for linux/darwin/windows (amd64/arm64).
# Usage:
#   build-artifacts.sh <signing-key> <out-dir> [target]
#   build-artifacts.sh watch
# target is linux-amd64 (default) or all.
set -euo pipefail

AGENT_SRC="${AGENT_SRC:-/app}"
WATCH=0
if [ "${1:-}" = "watch" ]; then
	WATCH=1
	KEY="${SIGNING_KEY:-/keys/core-signing.key}"
	OUT="${ARTIFACT_DIR:-/artifacts}"
	TARGET="${TARGETS:-all}"
else
	KEY="${1:?signing key path required}"
	OUT="${2:?output dir required}"
	TARGET="${3:-linux-amd64}"
fi

# Old callers passed a bare arch (amd64 / arm64).
case "$TARGET" in
amd64 | arm64) TARGET="linux-$TARGET" ;;
esac

wait_for_key() {
	i=0
	while [ ! -s "$KEY" ]; do
		i=$((i + 1))
		if [ "$i" -ge 90 ]; then
			echo "timed out waiting for $KEY" >&2
			exit 1
		fi
		sleep 1
	done
}

host_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) echo amd64 ;;
	aarch64 | arm64) echo arm64 ;;
	*) echo unknown ;;
	esac
}

pip_platforms() {
	case "$1" in
	linux-amd64) echo manylinux_2_17_x86_64 manylinux2014_x86_64 ;;
	linux-arm64) echo manylinux_2_17_aarch64 manylinux2014_aarch64 ;;
	darwin-amd64) echo macosx_11_0_x86_64 macosx_10_9_x86_64 ;;
	darwin-arm64) echo macosx_11_0_arm64 ;;
	windows-amd64) echo win_amd64 ;;
	windows-arm64) echo win_arm64 ;;
	*) return 1 ;;
	esac
}

write_unix_launcher() {
	cat >"$1" <<'LAUNCHER'
#!/bin/sh
DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
if [ -d "$DIR/site" ]; then
	export PYTHONPATH="$DIR/site"
	exec python3 -m autosecrets_agent.cli "$@"
fi
if [ ! -x "$DIR/venv/bin/python3" ]; then
	python3 -m venv "$DIR/venv"
	"$DIR/venv/bin/python3" -m pip install --disable-pip-version-check -q \
		--no-index --find-links "$DIR/wheelhouse" autosecrets-agent
fi
exec "$DIR/venv/bin/python3" -m autosecrets_agent.cli "$@"
LAUNCHER
	chmod 0755 "$1"
}

write_windows_launcher() {
	cat >"$1" <<'LAUNCHER'
@echo off
setlocal
set DIR=%~dp0
if exist "%DIR%site\" (
  set PYTHONPATH=%DIR%site
  python -m autosecrets_agent.cli %*
  exit /b %ERRORLEVEL%
)
if not exist "%DIR%venv\Scripts\python.exe" (
  python -m venv "%DIR%venv"
  "%DIR%venv\Scripts\python.exe" -m pip install --disable-pip-version-check -q --no-index --find-links "%DIR%wheelhouse" autosecrets-agent
)
"%DIR%venv\Scripts\python.exe" -m autosecrets_agent.cli %*
LAUNCHER
}

sign_file() {
	openssl pkeyutl -sign -rawin -inkey "$KEY" -in "$1" -out "$1.sig"
}

artifact_ok() {
	art="$1"
	[ -f "$art" ] && [ -f "$art.sig" ] || return 1
	openssl pkeyutl -verify -rawin -pubin -inkey "$PUB" -sigfile "$art.sig" -in "$art" >/dev/null 2>&1
}

# Content fingerprint of the packaged agent. Existing tarballs stay "up to
# date" across image rebuilds unless this changes, which is why a Core
# update could ship a new identity check with yesterday's Agent package.
source_hash() {
	tar -C "$AGENT_SRC" -cf - src pyproject.toml 2>/dev/null | sha256sum | awk '{print $1}'
}

invalidate_stale_artifacts() {
	current="$(source_hash)"
	stamp="$OUT/.src-hash"
	if [ -f "$stamp" ] && [ "$(cat "$stamp")" = "$current" ]; then
		return 0
	fi
	echo "agent source changed; rebuilding artifacts"
	rm -f "$OUT"/autosecrets-agent-*.tar.gz "$OUT"/autosecrets-agent-*.tar.gz.sig
	printf '%s\n' "$current" >"$stamp"
}

download_wheels() {
	dest="$1"
	shift
	for plat in "$@"; do
		if python3 -m pip download --disable-pip-version-check -q -d "$dest" \
			--platform "$plat" \
			--python-version 311 \
			--implementation cp \
			--abi cp311 \
			--only-binary=:all: \
			"cryptography>=43" "age==0.5.1"; then
			return 0
		fi
	done
	return 1
}

build_one() {
	osarch="$1"
	os="${osarch%-*}"
	arch="${osarch#*-}"
	name="autosecrets-agent-$os-$arch.tar.gz"
	art="$OUT/$name"
	if artifact_ok "$art"; then
		echo "up to date: $name"
		return 0
	fi
	plats="$(pip_platforms "$osarch")" || {
		echo "skip unknown target $osarch" >&2
		return 0
	}

	work="$(mktemp -d)"
	write_unix_launcher "$work/autosecrets-agent"
	if [ "$os" = windows ]; then
		write_windows_launcher "$work/autosecrets-agent.cmd"
	fi
	if [ "$os" = linux ] && [ "$arch" = "$(host_arch)" ]; then
		python3 -m pip install --disable-pip-version-check -q --target "$work/site" "$AGENT_SRC"
	else
		mkdir -p "$work/wheelhouse"
		python3 -m pip wheel --disable-pip-version-check -q --no-deps -w "$work/wheelhouse" "$AGENT_SRC"
		# shellcheck disable=SC2086
		if ! download_wheels "$work/wheelhouse" $plats; then
			echo "warn: no wheels for $osarch; skip" >&2
			rm -rf "$work"
			return 0
		fi
	fi
	mkdir -p "$OUT"
	tar -czf "$art" -C "$work" .
	sign_file "$art"
	rm -rf "$work"
	echo "artifact: $art ($(wc -c <"$art") bytes)"
}

build_targets() {
	mkdir -p "$OUT"
	invalidate_stale_artifacts
	PUB="$(mktemp)"
	openssl pkey -in "$KEY" -pubout -out "$PUB"
	if [ "$TARGET" = all ]; then
		set -- linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 \
			windows-amd64 windows-arm64
	else
		set -- "$TARGET"
	fi
	for t in "$@"; do
		build_one "$t"
	done
	rm -f "$PUB"
}

wait_for_key
build_targets

if [ "$WATCH" = 1 ]; then
	echo "watching $KEY and agent source for changes"
	last_key="$(stat -c %Y "$KEY" 2>/dev/null || stat -f %m "$KEY")"
	last_src="$(source_hash)"
	while true; do
		sleep 30
		now_key="$(stat -c %Y "$KEY" 2>/dev/null || stat -f %m "$KEY")"
		now_src="$(source_hash)"
		if [ "$now_key" != "$last_key" ] || [ "$now_src" != "$last_src" ]; then
			echo "signing key or agent source changed; rebuilding"
			last_key="$now_key"
			last_src="$now_src"
			build_targets
		fi
	done
fi
