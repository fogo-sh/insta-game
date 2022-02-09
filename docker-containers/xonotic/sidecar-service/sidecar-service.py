from enum import Enum
from functools import wraps
import os
import signal
import subprocess
from subprocess import PIPE
from datetime import datetime
import sys
from typing import Optional

import requests
from flask import Flask, request, abort

DEBUG = bool(os.environ.get("DEBUG", False))
CONFIG_URL = os.environ.get(
    "CONFIG_URL",
    "https://raw.githubusercontent.com/xonotic/xonotic/master/server/server.cfg",
)

HOST = os.environ.get("HOST", "0.0.0.0")

DEFAULT_PORT = 5001
PORT = int(os.environ.get("PORT", DEFAULT_PORT))

TOKEN = os.environ.get("TOKEN", "abc123")

app = Flask(__name__)
process = None


def log(message):
    print(f"SIDECAR: {message}")


def get_game_pid() -> Optional[int]:
    try:
        int(subprocess.check_output(["pidof", "./xonotic-linux64-dedicated"]))
    except:
        log("Failed to get game pid, assuming not currently running")
        return None


def write_config():
    with open("/opt/server/server.cfg", "wb") as f:
        f.write(requests.get(CONFIG_URL).content)


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
    else:
        log("Process already running, shutting down")
        exit_game()
        log("Process shutdown, starting fresh process")
        process = fresh()
        log("Fresh process started")
        return Response.RESTARTED


def exit_game():
    global process
    process.stdin.write(str.encode("exit\n"))
    process.stdin.flush()
    process.wait(15)


def authorize(f):
    @wraps(f)
    def decorated_function(*args, **kwargs):
        if not "Authorization" in request.headers:
            log("Authorization header missing in request")
            abort(401)

        data = request.headers.get('Authorization')
        token = str.replace(str(data), "Bearer ", "")

        if token != TOKEN:
            log("Invalid token provided")
            abort(401)

        return f(*args, **kwargs)

    return decorated_function


@app.route("/")
def home():
    global process
    return {"pid": process.pid, "timestamp": datetime.now().isoformat()}


@app.route("/restart")
@authorize
def restart():
    global process
    response = start_or_restart_game()
    return {"pid": process.pid, "status": response.value}


if __name__ == "__main__":
    write_config()
    start_or_restart_game()

    def handle_signal(_signo, _stack_frame):
        log("Handling signal to exit...")
        exit_game()
        sys.exit(0)

    signal.signal(signal.SIGINT, handle_signal)
    signal.signal(signal.SIGTERM, handle_signal)

    log(f"Starting sidecar-service on {HOST}:{PORT}")
    app.run(debug=DEBUG, host=HOST, port=PORT)
