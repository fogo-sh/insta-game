#!/usr/bin/env sh

set -e

CACHE_DIR=.buildx-cache
NEW_CACHE_DIR=.buildx-cache-new

docker buildx build \
  --platform linux/arm64 \
  --cache-from type=local,src="$CACHE_DIR" \
  --cache-to type=local,dest="$NEW_CACHE_DIR",mode=max \
  -t xonotic-arm:latest \
  .

rm -rf "$CACHE_DIR"
mv "$NEW_CACHE_DIR" "$CACHE_DIR"
