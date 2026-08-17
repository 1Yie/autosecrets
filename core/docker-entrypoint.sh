#!/bin/sh
set -eu

if [ "$(id -u)" = 0 ]; then
	keys_dir="${CORE_KEYS_DIR:-/keys}"
	mkdir -p "$keys_dir"
	chown -R autosecrets:autosecrets "$keys_dir"
	exec su-exec autosecrets:autosecrets \
		/usr/local/bin/autosecrets-core
fi

exec /usr/local/bin/autosecrets-core
