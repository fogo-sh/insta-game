#!/bin/sh
# Copy game libs into baseq2 dir at startup — needed because the cache volume
# mounts over /opt/baseq2, shadowing anything baked into the image there.
cp /opt/baseq2-libs/*.so /opt/baseq2/ 2>/dev/null || true
exec /usr/local/bin/sidecar "$@"
