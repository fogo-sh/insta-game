import io
import os
import shutil
import signal
import socket
import subprocess
import sys
import threading
import zipfile
from datetime import datetime
from enum import Enum
from functools import wraps
from subprocess import PIPE
from time import sleep
from typing import Optional

import boto3
import requests
from flask import Flask, abort, request
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry

DEBUG = bool(os.environ.get("DEBUG", False))
DATA_URL = os.environ.get("DATA_URL", "")
DEFAULT_CONFIG_PATH = "/opt/default-server.cfg"
AWS_REGION = os.environ.get("AWS_REGION", "ca-central-1")
ECS_CLUSTER = os.environ.get("ECS_CLUSTER", "insta-game-cluster")
ECS_SERVICE = os.environ.get("ECS_SERVICE", "")
IDLE_TIMEOUT = int(os.environ.get("IDLE_TIMEOUT_SECONDS", "600"))
GAME_PORT = int(os.environ.get("GAME_PORT", "26000"))
QUAKE_BASEDIR = os.environ.get("QUAKE_BASEDIR", "/opt")
QUAKE_GAME = os.environ.get("QUAKE_GAME", "id1")
MAX_PLAYERS = int(os.environ.get("MAX_PLAYERS", "12"))

HOST = os.environ.get("HOST", "0.0.0.0")
DEFAULT_PORT = 5001
PORT = int(os.environ.get("PORT", DEFAULT_PORT))
TOKEN = os.environ.get("TOKEN", "abc123")
DOWNLOAD_USER_AGENT = os.environ.get(
    "DOWNLOAD_USER_AGENT",
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "
    "(KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
)

app = Flask(__name__)
process = None


def log(message):
    print(f"SIDECAR: {message}")


def get_server_info() -> Optional[dict]:
    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        sock.settimeout(2)
        # Quake 1 CCREQ_SERVER_INFO: 0x8000 control packet, 12-byte length,
        # opcode 0x02, game name "QUAKE", protocol version 3.
        sock.sendto(
            b"\x80\x00\x00\x0c\x02QUAKE\x00\x03",
            ("127.0.0.1", GAME_PORT),
        )
        data, _ = sock.recvfrom(4096)
        sock.close()

        if len(data) < 7 or data[:2] != b"\x80\x00" or data[4] != 0x83:
            return None

        payload = data[5:]
        parts = payload.split(b"\x00")
        if len(parts) < 3:
            return None

        hostname = parts[1].decode("utf-8", errors="ignore")
        map_name = parts[2].decode("utf-8", errors="ignore")
        current_players = str(payload[-3])
        max_players = str(payload[-2])

        return {
            "hostname": hostname,
            "map": map_name,
            "clients": current_players,
            "max_clients": max_players,
        }
    except Exception:
        return None


def get_player_count() -> int:
    info = get_server_info()
    if not info:
        return 0
    return int(info.get("clients", 0))


def fetch_data(url: str) -> bytes:
    session = requests.Session()
    retry = Retry(
        total=5,
        connect=5,
        read=5,
        backoff_factor=1,
        status_forcelist=(429, 500, 502, 503, 504),
        allowed_methods=("GET",),
    )
    session.mount("https://", HTTPAdapter(max_retries=retry))
    session.mount("http://", HTTPAdapter(max_retries=retry))

    response = session.get(
        url,
        headers={"User-Agent": DOWNLOAD_USER_AGENT},
        timeout=(15, 120),
    )
    response.raise_for_status()
    return response.content


def download_data():
    entries = [e.strip() for e in DATA_URL.split(";") if e.strip()]
    config_path = os.path.join(QUAKE_BASEDIR, QUAKE_GAME, "server.cfg")

    if entries:
        for entry in entries:
            url, _, path = entry.partition("=")
            url = url.strip()
            path = path.strip()
            log(f"Downloading data from: {url}")
            content = fetch_data(url)
            data = io.BytesIO(content)
            if zipfile.is_zipfile(data):
                data.seek(0)
                with zipfile.ZipFile(data) as zf:
                    zf.extractall(path or QUAKE_BASEDIR)
                log(f"Data extracted to {path or QUAKE_BASEDIR}")
            else:
                with open(path, "wb") as f:
                    f.write(content)
                log(f"Data saved to {path}")

        if not os.path.exists(config_path):
            log(f"No server.cfg found in downloaded data, using {DEFAULT_CONFIG_PATH}")
            shutil.copyfile(DEFAULT_CONFIG_PATH, config_path)
        return

    log(f"No DATA_URL set, using bundled default config: {DEFAULT_CONFIG_PATH}")
    shutil.copyfile(DEFAULT_CONFIG_PATH, config_path)


def shutdown_ecs_service():
    if not ECS_SERVICE:
        log("ECS_SERVICE not set, cannot auto-shutdown")
        return
    try:
        log(f"Attempting ECS shutdown for {ECS_CLUSTER}/{ECS_SERVICE} in {AWS_REGION}")
        ecs = boto3.client("ecs", region_name=AWS_REGION)
        ecs.update_service(cluster=ECS_CLUSTER, service=ECS_SERVICE, desiredCount=0)
        log("Auto-shutdown triggered successfully")
    except Exception as e:
        log(f"Failed to trigger auto-shutdown: {e}")


def auto_shutdown_loop():
    last_active = datetime.now()
    while True:
        sleep(60)
        count = get_player_count()
        if count > 0:
            last_active = datetime.now()
            log(f"Auto-shutdown: {count} player(s) active")
        else:
            idle_seconds = (datetime.now() - last_active).total_seconds()
            log(
                f"Auto-shutdown: server idle for {int(idle_seconds)}s (timeout: {IDLE_TIMEOUT}s)"
            )
            if idle_seconds >= IDLE_TIMEOUT:
                log("Idle timeout reached, triggering shutdown")
                shutdown_ecs_service()
                break


class Response(Enum):
    STARTED = "started"
    RESTARTED = "restarted"


def start_or_restart_game() -> Response:
    global process

    def fresh():
        return subprocess.Popen(
            [
                "./qssm",
                "-dedicated",
                str(MAX_PLAYERS),
                "-basedir",
                QUAKE_BASEDIR,
                "-game",
                QUAKE_GAME,
                "-port",
                str(GAME_PORT),
                "+exec",
                "server.cfg",
            ],
            stdin=PIPE,
        )

    if process is None:
        log("No process, starting initial process")
        process = fresh()
        return Response.STARTED

    log("Process already running, shutting down")
    exit_game()
    log("Process shutdown, starting fresh process")
    process = fresh()
    log("Fresh process started")
    return Response.RESTARTED


def exit_game():
    global process
    if process is None:
        return

    if process.poll() is not None:
        log("Game process already exited")
        process = None
        return

    try:
        process.stdin.write(b"quit\n")
        process.stdin.flush()
        process.wait(15)
    except BrokenPipeError:
        log("Game process stdin already closed during shutdown")
    except subprocess.TimeoutExpired:
        log("Game process did not exit cleanly, terminating")
        process.terminate()
        process.wait(5)
    finally:
        process = None


def authorize(f):
    @wraps(f)
    def decorated_function(*args, **kwargs):
        if "Authorization" not in request.headers:
            log("Authorization header missing in request")
            abort(401)

        data = request.headers.get("Authorization")
        token = str.replace(str(data), "Bearer ", "")

        if token != TOKEN:
            log("Invalid token provided")
            abort(401)

        return f(*args, **kwargs)

    return decorated_function


@app.route("/status")
def status():
    global process
    info = get_server_info()
    return {
        "running": process is not None and process.poll() is None,
        "ready": info is not None,
        "players": int((info or {}).get("clients", 0)),
        "hostname": (info or {}).get("hostname", ""),
        "map": (info or {}).get("map", ""),
        "timestamp": datetime.now().isoformat(),
    }


@app.route("/restart", methods=["GET", "POST"])
@authorize
def restart():
    global process
    response = start_or_restart_game()
    return {"pid": process.pid, "status": response.value}


@app.route("/stop", methods=["POST"])
@authorize
def stop():
    if process is not None:
        exit_game()
    return {"status": "stopped"}


if __name__ == "__main__":
    download_data()
    start_or_restart_game()

    if IDLE_TIMEOUT > 0 and ECS_SERVICE:
        shutdown_thread = threading.Thread(target=auto_shutdown_loop, daemon=True)
        shutdown_thread.start()
        log(f"Auto-shutdown enabled: idle timeout {IDLE_TIMEOUT}s")

    def handle_signal(_signo, _stack_frame):
        log("Handling signal to exit...")
        exit_game()
        sys.exit(0)

    signal.signal(signal.SIGINT, handle_signal)
    signal.signal(signal.SIGTERM, handle_signal)

    log(f"Starting sidecar-service on {HOST}:{PORT}")
    app.run(debug=DEBUG, host=HOST, port=PORT)
