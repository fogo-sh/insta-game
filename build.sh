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
    # Load saved DATA_URL from .env if present
    if [ -f .env ]; then
      . ./.env
    fi

    if [ -z "$QSSM_DATA_URL" ]; then
      printf "QSSM_DATA_URL (semicolon-separated url=path entries): "
      read -r QSSM_DATA_URL
      if [ -z "$QSSM_DATA_URL" ]; then
        echo "QSSM_DATA_URL is required to build QSS-M."
        exit 1
      fi
      echo "QSSM_DATA_URL=$QSSM_DATA_URL" >> .env
      echo "==> Saved QSSM_DATA_URL to .env"
    else
      echo "==> Using saved QSSM_DATA_URL from .env"
    fi

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
