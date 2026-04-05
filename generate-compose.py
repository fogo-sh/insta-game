#!/usr/bin/env python3
"""Generate compose.yml from docker-containers/*/game.json metadata.

Usage:
    python3 generate-compose.py           # write compose.yml
    python3 generate-compose.py --check   # exit 1 if compose.yml is out of date
"""

import json
import os
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).parent
DOCKER_CONTAINERS = REPO_ROOT / "docker-containers"
COMPOSE_PATH = REPO_ROOT / "compose.yml"


def load_games() -> list[dict]:
    games = []
    for entry in sorted(DOCKER_CONTAINERS.iterdir()):
        if not entry.is_dir():
            continue
        meta_path = entry / "game.json"
        dockerfile_path = entry / "Dockerfile"
        if not meta_path.exists() or not dockerfile_path.exists():
            continue
        games.append(json.loads(meta_path.read_text()))
    return games


def port_lines(ports: dict) -> list[str]:
    """Convert game.json ports dict to compose port strings."""
    lines = []
    for container_port, binding in ports.items():
        host_port = binding["hostPort"]
        # container_port is e.g. "26000/udp" or "5001/tcp"
        lines.append(f'"127.0.0.1:{host_port}:{container_port}"')
    return lines


def game_service(g: dict) -> str:
    game_id = g["id"]
    image = g.get("image", f"ghcr.io/fogo-sh/insta-game:{game_id}")
    ports = g.get("ports", {})
    volumes = g.get("volumes", [])
    data_url_env = g.get("dataUrlEnv")

    lines = []
    lines.append(f"  {game_id}:")
    lines.append(f"    image: {image}")
    lines.append(f"    build:")
    lines.append(f"      context: .")
    lines.append(f"      dockerfile: docker-containers/{game_id}/Dockerfile")
    lines.append(f"      platforms:")
    lines.append(f"        - linux/arm64")

    # ports
    lines.append(f"    ports:")
    for port_str in port_lines(ports):
        lines.append(f"      - {port_str}")

    # environment
    lines.append(f"    environment:")
    lines.append(f'      GAME_CMD: "{g["gameCmd"]}"')
    game_args = g.get("gameArgs", "")
    lines.append(f'      GAME_ARGS: "{game_args}"')
    lines.append(f'      GAME_QUIT_CMD: "{g.get("gameQuitCmd", "quit")}"')
    lines.append(f'      GAME_QUIT_TIMEOUT: "{g.get("gameQuitTimeout", 15)}"')
    if g.get("gamePort"):
        lines.append(f'      GAME_PORT: "{g["gamePort"]}"')
    if g.get("configPath"):
        lines.append(f'      CONFIG_PATH: "{g["configPath"]}"')
    lines.append(f'      RCON_PASSWORD: "${{RCON_PASSWORD:-abc123}}"')
    lines.append(f'      TOKEN: "abc123"')
    if data_url_env:
        lines.append(f'      DATA_URL: "${{{data_url_env}}}"')

    # volumes
    if volumes:
        lines.append(f"    volumes:")
        for vol in volumes:
            # normalise relative paths to use ./ prefix
            host, _, rest = vol.partition(":")
            if not host.startswith("/") and not host.startswith("./"):
                host = f"./{host}"
            lines.append(f"      - {host}:{rest}")

    return "\n".join(lines)


def launcher_data_url_env_vars(games: list[dict]) -> list[str]:
    """Collect all dataUrlEnv vars from games for the launcher service."""
    return [g["dataUrlEnv"] for g in games if g.get("dataUrlEnv")]


def generate(games: list[dict]) -> str:
    data_url_vars = launcher_data_url_env_vars(games)

    parts = ["services:"]

    for g in games:
        parts.append(game_service(g))
        parts.append("")  # blank line between services

    # launcher service — static, not derived from game.json
    launcher_lines = [
        "  launcher:",
        "    build:",
        "      context: ./launcher",
        "      dockerfile: Dockerfile",
        "    extra_hosts:",
        '      - "host.docker.internal:host-gateway"',
        "    volumes:",
        "      - /var/run/docker.sock:/var/run/docker.sock",
        "    ports:",
        '      - "127.0.0.1:3000:3000/tcp"',
        "    environment:",
        '      BACKEND: "docker"',
        '      DATA_DIR: "${PWD}"',
        '      PORT: "3000"',
        '      SIDECAR_HOST: "host.docker.internal"',
        '      WEB_UI_PASSPHRASE: "${WEB_UI_PASSPHRASE:-gaming}"',
        '      API_TOKEN: "${API_TOKEN:-gaming}"',
        '      SIDECAR_TOKEN: "${SIDECAR_TOKEN:-abc123}"',
        '      RCON_PASSWORD: "${RCON_PASSWORD:-abc123}"',
    ]
    for var in data_url_vars:
        launcher_lines.append(f'      {var}: "${{{var}:-}}"')

    parts.append("\n".join(launcher_lines))

    return "\n".join(parts) + "\n"


def main():
    check_mode = "--check" in sys.argv
    games = load_games()
    output = generate(games)

    if check_mode:
        current = COMPOSE_PATH.read_text() if COMPOSE_PATH.exists() else ""
        if current != output:
            print("compose.yml is out of date. Run: python3 generate-compose.py", file=sys.stderr)
            sys.exit(1)
        print("compose.yml is up to date.")
    else:
        COMPOSE_PATH.write_text(output)
        print(f"Generated compose.yml with {len(games)} game(s): {', '.join(g['id'] for g in games)}")


if __name__ == "__main__":
    main()
