#!/usr/bin/env sh

docker buildx build --platform linux/arm64 -t xonotic-arm:latest .
