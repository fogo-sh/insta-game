"""Exercise an empty, isolated AWS fleet; optionally join with real game clients."""

import argparse
import concurrent.futures
import json
import os
from pathlib import Path
import re
import subprocess
import time
import threading
import urllib.error
import urllib.request

parser = argparse.ArgumentParser()
parser.add_argument("--config", type=Path, required=True)
parser.add_argument("--key-file", type=Path, required=True)
parser.add_argument("--out", type=Path, required=True)
parser.add_argument(
    "--clients",
    action="store_true",
    help="Use scripts/benchmark/client.py and locally installed client fixtures",
)
args = parser.parse_args()
config = json.loads(args.config.read_text())
if not config.get("Fleet") or not re.fullmatch(
    r"arn:aws[a-z-]*:cloudformation:[a-z0-9-]+:\d{12}:stack/insta-game-capacity-[A-Za-z0-9-]+/[A-Za-z0-9-]+",
    config.get("Stack", ""),
):
    parser.error("use an isolated insta-game-capacity-* fleet")
url = config.get("LobbyURL", "").rstrip("/")
if not url.startswith("https://"):
    parser.error("the fleet needs a deployed HTTPS launch page")
args.out = args.out.resolve()
args.out.mkdir(parents=True, mode=0o700, exist_ok=False)
key = args.key_file.read_text().strip()
repo = Path(__file__).resolve().parents[1]
created = []
results = []
save_lock = threading.Lock()


def save(name, value):
    with save_lock:
        (args.out / name).write_text(json.dumps(value, indent=2))


def api(path="/api/games", body=None):
    for attempt in range(3):
        request = urllib.request.Request(
            url + path,
            data=json.dumps(body).encode() if body is not None else None,
            headers={
                "Authorization": "Bearer " + key,
                "Content-Type": "application/json",
            },
        )
        try:
            with urllib.request.urlopen(request, timeout=60) as response:
                return response.status, json.load(response)
        except urllib.error.HTTPError as error:
            response = json.load(error)
            if (
                error.code not in (429, 503)
                or str(response.get("error", "")).startswith("Host limit reached")
                or attempt == 2
            ):
                return error.code, response
            time.sleep(attempt + 1)
    raise RuntimeError("request retries exhausted")


def view():
    status, body = api()
    if status != 200:
        raise RuntimeError((status, body))
    return body


def wait_for(check, label, seconds=1200):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        body = view()
        result = check(body)
        if result:
            return result
        time.sleep(3)
    raise RuntimeError("Timed out: " + label)


def ready(server_id):
    def check(body):
        server = next((g for g in body["games"] if g["id"] == server_id), None)
        if server and server.get("error") and not server["pending"]:
            raise RuntimeError(server)
        return (
            server
            if server
            and server["phase"] == "running"
            and not server["pending"]
            and server.get("address")
            else None
        )

    return wait_for(check, server_id)


def create(game, duplicate=False):
    server_id = "server-" + os.urandom(16).hex()
    request = {"id": server_id, "game": game, "name": "Fleet check " + game}
    created.append(server_id)
    save("created.json", created)
    if duplicate:
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
            responses = list(pool.map(lambda _: api(body=request), range(2)))
    else:
        responses = [api(body=request)]
    return server_id, responses


def action(server_id, verb):
    status, body = api("/api/games/" + server_id + "/" + verb, {})
    if status != 202:
        raise RuntimeError((verb, status, body))


def remove(server_id):
    action(server_id, "remove")
    wait_for(
        lambda body: not any(g["id"] == server_id for g in body["games"]),
        "remove " + server_id,
        240,
    )


def join(game, server, label=None):
    if not args.clients:
        return False
    if server.get("players") != 0:
        raise RuntimeError("Client validation requires a zero-player fixture: " + game)
    output = args.out / ((label or game) + "-1")
    log = (args.out / ((label or game) + "-driver.log")).open("w")
    process = subprocess.Popen(
        [
            "python3",
            str(repo / "scripts/benchmark/client.py"),
            game,
            server["address"],
            "--out",
            str(output),
            "--seconds",
            "35",
        ],
        stdout=log,
        stderr=subprocess.STDOUT,
    )
    observations = []
    deadline = time.monotonic() + 180
    try:
        while process.poll() is None and time.monotonic() < deadline:
            current = next(g for g in view()["games"] if g["id"] == server["id"])
            observations.append(
                {
                    "time": time.time(),
                    "players": current.get("players"),
                    "phase": current["phase"],
                }
            )
            time.sleep(3)
        if (
            process.poll() is None
            or process.returncode != 0
            or not any((r["players"] or 0) > 0 for r in observations)
        ):
            raise RuntimeError("Real client did not join: " + game)
    finally:
        if process.poll() is None:
            process.terminate()
            process.wait(timeout=15)
        log.close()
        save((label or game) + "-observations.json", observations)
    print(game, "real client joined", flush=True)
    return True


try:
    initial = view()
    expected = {
        "bzflag",
        "fteqw",
        "q2repro",
        "ioquake3",
        "openarena",
        "ut99",
        "xonotic",
    }
    if (
        initial["games"]
        or initial["hosts"]
        or {g["id"] for g in initial["choices"]} != expected
        or initial["maxHosts"] != 2
    ):
        raise RuntimeError("use an empty, seven-game fleet with a two-host limit")
    save("empty-catalog.json", initial)
    keeper = host = None
    for game in [
        "bzflag",
        "fteqw",
        "q2repro",
        "ioquake3",
        "openarena",
        "ut99",
        "xonotic",
    ]:
        started = time.time()
        server_id, responses = create(game, duplicate=keeper is None)
        if any(status != 202 for status, _ in responses):
            raise RuntimeError(responses)
        server = ready(server_id)
        ready_seconds = time.time() - started
        if keeper is None:
            keeper, host = server_id, server["hostId"]
        if server["hostId"] != host or len(view()["hosts"]) != 1:
            raise RuntimeError("one host could not prepare every catalog game")
        client_joined = join(game, server)
        results.append(
            {
                "game": game,
                "id": server_id,
                "host": host,
                "ready_seconds": ready_seconds,
                "client_joined": client_joined,
            }
        )
        save("games.json", results)
        print(game, "ready on", host, flush=True)
        if server_id != keeper:
            remove(server_id)

    # Five Xonotic requests exceed two default small-host budgets, even without players.
    with concurrent.futures.ThreadPoolExecutor(max_workers=5) as pool:
        requests = list(pool.map(lambda _: create("xonotic"), range(5)))
    accepted = [
        server_id for server_id, responses in requests if responses[0][0] == 202
    ]
    refused = [responses[0] for _, responses in requests if responses[0][0] != 202]
    if (
        len(accepted) != 4
        or len(refused) != 1
        or refused[0][0] != 429
        or "Host limit reached" not in refused[0][1].get("error", "")
    ):
        raise RuntimeError(("capacity admission", requests))
    for server_id in accepted:
        ready(server_id)
    packed = view()
    if len(packed["hosts"]) != 2:
        raise RuntimeError("overflow failed to create exactly one additional host")
    save(
        "capacity.json",
        {"accepted": accepted, "refused": refused, "hosts": packed["hosts"]},
    )
    print("concurrent capacity admission passed; two hosts", flush=True)
    for server_id in accepted:
        remove(server_id)
    wait_for(lambda body: len(body["hosts"]) == 1, "empty host retirement", 240)

    action(keeper, "stop")
    wait_for(
        lambda body: len(body["hosts"]) == 1 and body["hosts"][0]["state"] == "stopped",
        "host sleep",
        360,
    )
    asleep = view()
    save("sleeping.json", asleep)
    action(keeper, "start")
    restarted = ready(keeper)
    if restarted["hostId"] != host:
        raise RuntimeError("restart moved saved server")
    join("bzflag", restarted, "bzflag-restarted")
    save(
        "result.json",
        {
            "games": results,
            "concurrent_duplicate": True,
            "capacity": True,
            "sleep_wake": True,
            "same_saved_assignment": True,
        },
    )
    print("fleet lifecycle passed", flush=True)
finally:
    # Touch only IDs allocated by this run. Provider termination still verifies ownership.
    cleanup_errors = []
    for server_id in created:
        try:
            if any(g["id"] == server_id for g in view()["games"]):
                action(server_id, "remove")
        except Exception as error:
            cleanup_errors.append(str(error))
    if created:
        try:
            wait_for(
                lambda body: not body["games"] and not body["hosts"],
                "test cleanup",
                300,
            )
        except Exception as error:
            cleanup_errors.append(str(error))
    save("cleanup.json", {"errors": cleanup_errors, "finished_at": time.time()})
    if cleanup_errors:
        raise RuntimeError(("cleanup incomplete", cleanup_errors))
