#!/usr/bin/env sh
set -e

GAME=$1

if [ -z "$GAME" ]; then
  echo "Usage: $0 <game>"
  echo "  Games: xonotic, qssm, q2repro"
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

    mkdir -p .cache/qssm
    echo "==> Building qssm image..."
    docker compose build qssm
    ;;
  q2repro)
    if [ -f .env ]; then
      . ./.env
    fi

    if [ -z "$Q2REPRO_DATA_URL" ]; then
      printf "Q2REPRO_DATA_URL (semicolon-separated url=path entries): "
      read -r Q2REPRO_DATA_URL
      if [ -z "$Q2REPRO_DATA_URL" ]; then
        echo "Q2REPRO_DATA_URL is required to run q2repro."
        exit 1
      fi
      echo "Q2REPRO_DATA_URL=$Q2REPRO_DATA_URL" >> .env
      echo "==> Saved Q2REPRO_DATA_URL to .env"
    else
      echo "==> Using saved Q2REPRO_DATA_URL from .env"
    fi

    mkdir -p .cache/q2repro
    echo "==> Building q2repro image..."
    docker compose build q2repro
    echo "==> Seeding cache with game libs from image..."
    id=$(docker create ghcr.io/fogo-sh/insta-game:q2repro)
    docker cp "$id:/opt/baseq2/." .cache/q2repro/
    docker rm "$id"
    ;;
  *)
    echo "Unknown game: $GAME"
    echo "  Games: xonotic, qssm, q2repro"
    exit 1
    ;;
esac

echo "==> Done."
