from datetime import datetime
from enum import Enum
from functools import wraps
import os
import shutil
import signal
import socket
import subprocess
from subprocess import PIPE
import sys
import threading
from time import sleep
from typing import Optional

import boto3
import requests
from flask import Flask, request, abort

DEBUG = bool(os.environ.get("DEBUG", False))
CONFIG_URL = os.environ.get("CONFIG_URL", "")
DEFAULT_CONFIG_PATH = "/opt/default-server.cfg"
AWS_REGION = os.environ.get("AWS_REGION", "ca-central-1")
ECS_CLUSTER = os.environ.get("ECS_CLUSTER", "insta-game-cluster")
ECS_SERVICE = os.environ.get("ECS_SERVICE", "")
IDLE_TIMEOUT = int(os.environ.get("IDLE_TIMEOUT_SECONDS", "600"))
GAME_PORT = int(os.environ.get("GAME_PORT", "26000"))

HOST = os.environ.get("HOST", "0.0.0.0")
DEFAULT_PORT = 5001
PORT = int(os.environ.get("PORT", DEFAULT_PORT))
TOKEN = os.environ.get("TOKEN", "abc123")

app = Flask(__name__)
process = None


def log(message):
    print(f"SIDECAR: {message}")


def get_server_info() -> Optional[dict]:
    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        sock.settimeout(2)
        sock.sendto(b"\xff\xff\xff\xffgetinfo", ("127.0.0.1", GAME_PORT))
        data, _ = sock.recvfrom(1024)
        sock.close()
        # Response format: \xff\xff\xff\xffinfoResponse\n\key\val\key\val\...
        payload = data[17:].decode("utf-8", errors="ignore")
        parts = payload.split("\\")
        info_dict = {}
        for i in range(1, len(parts) - 1, 2):
            info_dict[parts[i]] = parts[i + 1]
        return info_dict
    except Exception:
        return None


def get_player_count() -> int:
    info = get_server_info()
    if not info:
        return 0
    return int(info.get("clients", 0))


def write_config(config_url: Optional[str] = None):
    source_url = config_url or CONFIG_URL
    if source_url:
        log(f"Fetching config from URL: {source_url}")
        response = requests.get(source_url, timeout=30)
        response.raise_for_status()
        with open("/opt/data/server.cfg", "wb") as f:
            f.write(response.content)
        return

    log(f"Using bundled default config: {DEFAULT_CONFIG_PATH}")
    shutil.copyfile(DEFAULT_CONFIG_PATH, "/opt/data/server.cfg")


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
        return subprocess.Popen(["./xonotic-linux64-dedicated"], stdin=PIPE)

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
        process.stdin.write(str.encode("exit\n"))
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
        "timestamp": datetime.now().isoformat(),
    }


@app.route("/restart", methods=["GET", "POST"])
@authorize
def restart():
    global process
    payload = request.get_json(silent=True) or {}
    config_url = payload.get("config_url")
    if config_url:
        write_config(config_url=config_url)
    response = start_or_restart_game()
    return {"pid": process.pid, "status": response.value}


@app.route("/stop", methods=["POST"])
@authorize
def stop():
    if process is not None:
        exit_game()
    return {"status": "stopped"}


if __name__ == "__main__":
    write_config()
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
