#!/usr/bin/env sh
set -e

if [ "$#" -eq 0 ]; then
  echo "usage: start-ut99.sh <ucc-bin> [args...]" >&2
  exit 1
fi

server_bin=$1
shift

chmod +x "$server_bin" 2>/dev/null || true
cd "$(dirname "$server_bin")"
exec "$server_bin" "$@"
