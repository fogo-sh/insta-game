#!/usr/bin/env sh
set -e

GAME=$1
GAME_DIRS=$(find docker-containers -mindepth 2 -maxdepth 2 -type f -name game.json -exec dirname {} \; | while read -r dir; do [ -f "$dir/Dockerfile" ] && printf '%s\n' "$dir"; done | sort)
GAMES=$(printf '%s\n' "$GAME_DIRS" | sed '/^$/d' | xargs -r -n1 basename)
GAME_LIST=$(printf '%s\n' "$GAMES" | paste -sd ', ' -)

ensure_data_url() {
  game_name=$1
  env_name=$2
  prompt=$3
  empty_message=$4
  cache_dir=$5

  if [ -f .env ]; then
    # shellcheck disable=SC1091
    . ./.env
  fi

  eval current_value=\${$env_name}
  if [ -z "$current_value" ]; then
    printf "%s" "$prompt"
    read -r current_value
    if [ -z "$current_value" ]; then
      echo "$empty_message"
      exit 1
    fi
    echo "$env_name=$current_value" >> .env
    echo "==> Saved $env_name to .env"
  else
    echo "==> Using saved $env_name from .env"
  fi

  mkdir -p "$cache_dir"
}

prepare_game() {
  game_name=$1

  case "$game_name" in
    fteqw)
      ensure_data_url \
        "$game_name" \
        "FTEQW_DATA_URL" \
        "FTEQW_DATA_URL (semicolon-separated url=path entries): " \
        "FTEQW_DATA_URL is required to build FTEQW." \
        ".cache/fteqw"
      ;;
    q2repro)
      ensure_data_url \
        "$game_name" \
        "Q2REPRO_DATA_URL" \
        "Q2REPRO_DATA_URL (semicolon-separated url=path entries): " \
        "Q2REPRO_DATA_URL is required to run q2repro." \
        ".cache/q2repro"
      ;;
    ioquake3)
      ensure_data_url \
        "$game_name" \
        "IOQUAKE3_DATA_URL" \
        "IOQUAKE3_DATA_URL (zip URL or semicolon-separated url=path entries): " \
        "IOQUAKE3_DATA_URL is required to run ioquake3." \
        ".cache/ioquake3"
      ;;
    smokinguns)
      ensure_data_url \
        "$game_name" \
        "SMOKINGUNS_DATA_URL" \
        "SMOKINGUNS_DATA_URL (zip URL that extracts Smokin' Guns into /opt): " \
        "SMOKINGUNS_DATA_URL is required to run smokinguns." \
        ".cache/smokinguns"
      ;;
    bzflag)
      mkdir -p .cache/bzflag
      ;;
    ut99)
      ensure_data_url \
        "$game_name" \
        "UT99_DATA_URL" \
        "UT99_DATA_URL (zip URL that extracts into /opt): " \
        "UT99_DATA_URL is required to run ut99." \
        ".cache/ut99"
      ;;
  esac

  echo "==> Preparing $game_name build context..."
  make -C "docker-containers/$game_name" download clean
}

build_game_image() {
  game_name=$1

  echo "==> Building $game_name image..."
  docker buildx build \
    --platform linux/arm64 \
    --load \
    -t "ghcr.io/fogo-sh/insta-game:$game_name" \
    -f "docker-containers/$game_name/Dockerfile" \
    .
}

if [ -z "$GAME" ]; then
  echo "Usage: $0 <game>"
  echo "  Games: $GAME_LIST, launcher, all"
  exit 1
fi

case "$GAME" in
  all)
    echo "==> Building all services..."
    "$0" launcher
    for game_name in $GAMES; do
      "$0" "$game_name"
    done
    echo "==> All builds complete."
    exit 0
    ;;
  launcher)
    echo "==> Building launcher bundle..."
    cd launcher
    npm install
    npm run build:docker
    cd ..
    echo "==> Building launcher image..."
    docker compose build launcher
    ;;
  *)
    if printf '%s\n' "$GAMES" | grep -Fxq "$GAME"; then
      prepare_game "$GAME"
      build_game_image "$GAME"
    else
      echo "Unknown game: $GAME"
      echo "  Games: $GAME_LIST, launcher, all"
      exit 1
    fi
    ;;
esac

echo "==> Done."
