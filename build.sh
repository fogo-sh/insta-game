#!/usr/bin/env sh
set -e

GAME=$1

if [ -z "$GAME" ]; then
  echo "Usage: $0 <game>"
  echo "  Games: xonotic, qssm"
  exit 1
fi

case "$GAME" in
  xonotic)
    echo "==> Preparing Xonotic build context..."
    cd docker-containers/xonotic
    make download
    make clean
    cd ../..
    echo "==> Building xonotic image..."
    docker compose build xonotic
    ;;
  qssm)
    echo "==> Building qssm image..."
    docker compose build qssm
    ;;
  *)
    echo "Unknown game: $GAME"
    echo "  Games: xonotic, qssm"
    exit 1
    ;;
esac

echo "==> Done."
