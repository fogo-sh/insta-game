"""Run a bounded game-load matrix through an SSH-forwarded insta-game controller."""

import argparse
import concurrent.futures
import json
import os
import pathlib
import re
import signal
import subprocess
import time
import urllib.error
import urllib.request

from probe import probe, ut_bot_count

GAMES = ["fteqw", "q2repro", "ioquake3", "openarena", "xonotic", "ut99", "bzflag"]


def config(game, bots, repo):
    if game == "fteqw":
        return f'sv_listen_nq 1\nsv_public 0\nhostname "Capacity Quake"\ndeathmatch 1\nmaxclients 24\ntimelimit 0\nfraglimit 0\nskill 2\nsaved1 1\nscratch1 {2**bots - 1}\nscratch2 {sum(2 << (i * 2) for i in range(bots))}\nmap dm4\n'
    if game == "q2repro":
        return 'set hostname "Capacity Quake II"\nset deathmatch 1\nset maxclients 32\nset sv_iplimit 32\nset public 0\nset timelimit 0\nset fraglimit 0\nmap demo1\n'
    if game in ("ioquake3", "openarena"):
        return (
            'set sv_hostname "Capacity"\nset sv_maxclients 32\nset sv_pure 0\nset sv_fps 20\nset vm_game 0\nset g_gametype 0\nset bot_enable 1\nset bot_minplayers 0\nset g_spskill 4\nset timelimit 0\nset fraglimit 0\nmap '
            + ("q3dm1" if game == "ioquake3" else "oa_dm5")
            + "\n"
            + "".join(
                "addbot " + ("Sarge" if game == "ioquake3" else "Gargoyle") + " 4\n"
                for _ in range(bots)
            )
        )
    if game == "xonotic":
        return f'hostname "Capacity Xonotic"\nsv_public 0\nmaxplayers 32\ng_maxplayers 32\ng_start_delay 0\ng_warmup 0\nbot_join_empty 1\nbot_number {bots}\nminplayers {bots}\nskill 7\ntimelimit_override 0\nfraglimit_override 0\nmap stormkeep\n'
    if game == "ut99":
        data = (
            (repo / "docker-containers/ut99/UnrealTournament.ini")
            .read_text()
            .replace("MinPlayers=0", f"MinPlayers={bots}\nNetWait=-1")
            .replace("InitialBots=0", f"InitialBots={bots}")
            .replace("FragLimit=30", "FragLimit=0")
            .replace("TimeLimit=20", "TimeLimit=0")
        )
        return (
            data
            + "\n[Engine.GameInfo]\nMaxPlayers=32\nMaxSpectators=0\nbNoMonsters=True\n"
        )
    return "-advertise NONE\n-j\n+r\n-mp 32\n"


class Bench:
    def __init__(self, connection, out, repo):
        if not re.fullmatch(
            r"arn:aws:cloudformation:[a-z0-9-]+:[0-9]{12}:stack/insta-game-capacity-[a-z0-9-]+/[A-Za-z0-9-]+",
            connection.get("stack", ""),
        ):
            raise ValueError("an isolated insta-game-capacity stack ARN is required")
        if not connection["url"].startswith("http://127.0.0.1:"):
            raise ValueError("use a pinned SSH tunnel to the benchmark controller")
        self.connection = connection
        self.out = out
        self.repo = repo
        self.token = pathlib.Path(connection["token"]).read_text().strip()
        self.serial = 0

    def api(self, path="", method="GET", body=None):
        req = urllib.request.Request(
            self.connection["url"] + "/api/instances" + path,
            data=json.dumps(body).encode() if body is not None else None,
            method=method,
            headers={
                "Authorization": "Bearer " + self.token,
                "Content-Type": "application/json",
            },
        )
        with urllib.request.urlopen(req, timeout=40) as r:
            return json.load(r)

    def create(self, game, bots):
        self.serial += 1
        id = f"cap-{game}-{int(time.time())}-{self.serial}"
        self.api(
            "",
            "POST",
            {
                "id": id,
                "game": game,
                "name": id,
                "idleMinutes": 0,
                "maxMinutes": 180,
                "config": config(game, bots, self.repo),
            },
        )
        return id

    def wait(self, ids, phase, timeout=110):
        end = time.monotonic() + timeout
        while time.monotonic() < end:
            rows = {r["id"]: r for r in self.api()}
            if all(rows.get(id, {"phase": "deleted"})["phase"] == phase for id in ids):
                return [rows.get(id, {"id": id, "phase": "deleted"}) for id in ids]
            if phase == "running" and any(
                rows.get(id, {}).get("phase") == "failed" for id in ids
            ):
                raise RuntimeError(
                    str(
                        [
                            {k: rows[id].get(k) for k in ["id", "phase", "error"]}
                            for id in ids
                        ]
                    )
                )
            time.sleep(2)
        raise RuntimeError(
            "readiness timeout "
            + str(
                [
                    {k: rows[id].get(k) for k in ["id", "phase", "players", "error"]}
                    for id in ids
                ]
            )
        )

    def clear(self):
        ids = [
            r["id"]
            for r in self.api()
            if r["id"].startswith("cap-") and r["desired"] != "deleted"
        ]
        for id in ids:
            self.api("/" + id, "DELETE")
        if ids:
            self.wait(ids, "deleted", 150)

    def memory_guard(self):
        path = pathlib.Path(
            self.connection.get(
                "samples",
                pathlib.Path(self.connection["token"]).with_name("samples.jsonl"),
            )
        )
        try:
            with path.open("rb") as source:
                source.seek(0, 2)
                end = source.tell()
                source.seek(max(0, end - 65536))
                lines = source.read().splitlines()
            latest = json.loads(lines[-1])
            if time.time() - latest["time"] > 15:
                raise RuntimeError("host telemetry is stale")
            if latest["memory_available"] < max(
                128 * 1024**2, 0.12 * latest["memory_total"]
            ):
                raise RuntimeError(
                    "memory safety guard: less than 12% or 128 MiB available"
                )
        except (FileNotFoundError, IndexError, json.JSONDecodeError) as error:
            raise RuntimeError("host telemetry is unavailable") from error

    def scenario(
        self,
        loads,
        label,
        seconds=60,
        warmup=20,
        network_clients=0,
        robots=0,
        spread_clients=False,
    ):
        run_id = f"{self.connection['tier']}-{int(time.time())}-{label}"
        meta = {
            "run_id": run_id,
            "tier": self.connection["tier"],
            "instance_id": self.connection["instance_id"],
            "label": label,
            "loads": loads,
            "seconds": seconds,
            "warmup": warmup,
            "started_at": time.time(),
            "load_kind": "engine-bots" if any(b for g, b in loads) else "idle",
            "network_clients": network_clients + robots,
            "status": "running",
        }
        folder = self.out / run_id
        folder.mkdir(parents=True)
        ids = []
        clients = []

        def save():
            (folder / "run.json").write_text(json.dumps(meta, indent=2))

        save()
        try:
            preflight_deadline = time.monotonic() + 45
            while True:
                try:
                    self.memory_guard()
                    break
                except RuntimeError:
                    if time.monotonic() >= preflight_deadline:
                        raise
                    time.sleep(3)
            for game, bots in loads:
                self.memory_guard()
                ids.append(
                    self.create(
                        game,
                        bots + network_clients
                        if game == "ut99" and network_clients
                        else bots,
                    )
                )
                self.wait(ids[-1:], "running")
                time.sleep(2)
            instances = self.wait(ids, "running")
            if network_clients:
                meta["load_kind"] = (
                    "network-clients" if not any(b for g, b in loads) else "hybrid"
                )
                targets = instances if spread_clients else instances[:1]
                meta["network_clients"] = len(targets) * (network_clients + robots)
                for target in targets:
                    address = next(
                        e["address"]
                        for e in target["endpoints"]
                        if e["port"] == target["game"]["gamePort"]
                    )
                    for n in range(network_clients):
                        index = len(clients)
                        log = open(folder / f"client-{index}-driver.log", "w")
                        process = subprocess.Popen(
                            [
                                "python3",
                                str(self.repo / "scripts/benchmark/client.py"),
                                target["game"]["id"],
                                address,
                                "--out",
                                str(folder / f"client-{index}"),
                                "--seconds",
                                str(seconds + warmup + 180),
                                "--robots",
                                str(robots),
                            ],
                            stdout=log,
                            stderr=subprocess.STDOUT,
                            start_new_session=True,
                        )
                        clients.append((process, log))
                        time.sleep(6 if target["game"]["id"] == "xonotic" else 2)
                end = time.monotonic() + 90
                while time.monotonic() < end:
                    populations = {
                        target["id"]: self.api("/" + target["id"]).get("players", 0)
                        for target in targets
                    }
                    if all(
                        populations[target["id"]]
                        >= network_clients
                        + robots
                        + (
                            target.get("players", 0)
                            if target["game"]["id"] == "fteqw"
                            else 0
                        )
                        for target in targets
                    ):
                        break
                    if any(process.poll() is not None for process, log in clients):
                        raise RuntimeError("network client exited before joining")
                    time.sleep(2)
                else:
                    raise RuntimeError(
                        "network clients did not join: " + str(populations)
                    )
            time.sleep(warmup)
            instances = self.wait(ids, "running")
            meta["instances"] = [
                {k: i[k] for k in ["id", "game", "players", "endpoints"]}
                for i in instances
            ]
            endpoints = []
            for i, (game, bots) in zip(instances, loads):
                logical = i["game"]["gamePort"] + (1 if game == "ut99" else 0)
                ep = next(
                    e
                    for e in i["endpoints"]
                    if e["port"] == logical
                    and (e["protocol"] == "udp" or game == "bzflag")
                )
                endpoints.append((game, ep["address"], i["id"]))
            observed = [probe(game, address) for game, address, id in endpoints]
            meta["initial_probes"] = observed
            counts = []
            for id, (game, bots), result in zip(ids, loads, observed):
                if game == "fteqw":
                    logs = subprocess.check_output(
                        self.connection["ssh"] + ["sudo docker logs insta-game-" + id],
                        text=True,
                        stderr=subprocess.STDOUT,
                    )
                    counts.append(
                        len(
                            set(
                                name
                                for name in re.findall(r"(.+) entered the game", logs)
                                if not name.startswith("Load")
                            )
                        )
                    )
                elif game == "ut99":
                    counts.append(
                        ut_bot_count(
                            next(
                                address
                                for g, address, server_id in endpoints
                                if server_id == id
                            )
                        )
                    )
                else:
                    counts.append(result.get("players", 0))
            meta["verified_population"] = dict(zip(ids, counts))
            mismatches = [
                (id, bots, count)
                for (game, address, id), (g, bots), result, count in zip(
                    endpoints, loads, observed, counts
                )
                if bots and count < bots
            ]
            if mismatches:
                raise RuntimeError("bot count mismatch " + str(mismatches))
            # Avoid a back-to-back query burst after population validation.
            time.sleep(2)
            meta["measured_at"] = time.time()
            end = time.monotonic() + seconds
            with (
                (folder / "probes.jsonl").open("w") as output,
                concurrent.futures.ThreadPoolExecutor(max_workers=16) as pool,
            ):
                while time.monotonic() < end:
                    self.memory_guard()
                    started = time.time()
                    for (game, address, id), result in zip(
                        endpoints, pool.map(lambda e: probe(e[0], e[1]), endpoints)
                    ):
                        output.write(
                            json.dumps(
                                {
                                    "time": started,
                                    "run_id": run_id,
                                    "server_id": id,
                                    "game": game,
                                    **result,
                                }
                            )
                            + "\n"
                        )
                    output.flush()
                    time.sleep(max(0, 2 - (time.time() - started)))
            meta["measured_until"] = time.time()
            meta["status"] = "measured"
            final = self.api()
            meta["final_players"] = {
                i["id"]: i.get("players") for i in final if i["id"] in ids
            }
            if network_clients and any(
                (meta["final_players"].get(target["id"]) or 0)
                < network_clients + robots
                for target in targets
            ):
                raise RuntimeError("network client disconnected during measurement")
            if network_clients and any(
                process.poll() is not None for process, log in clients
            ):
                raise RuntimeError("network client exited during measurement")
            for n, (process, log) in enumerate(clients):
                display_file = folder / f"client-{n}" / "display"
                if display_file.exists():
                    subprocess.run(
                        [
                            "import",
                            "-display",
                            display_file.read_text().strip(),
                            "-window",
                            "root",
                            str(folder / f"client-{n}" / "measured.png"),
                        ],
                        check=True,
                    )

        except KeyboardInterrupt:
            meta["status"] = "interrupted"
            meta["error"] = "Benchmark worker interrupted"
            raise
        except Exception as error:
            meta["status"] = "failed"
            meta["error"] = str(error)
        finally:
            for process, log in clients:
                if process.poll() is None:
                    os.killpg(process.pid, signal.SIGTERM)
                try:
                    process.wait(timeout=15)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                log.close()
            for id in ids:
                with (folder / (id + ".log")).open("w") as log:
                    try:
                        subprocess.run(
                            self.connection["ssh"]
                            + [
                                "sudo docker logs --timestamps --tail 150 insta-game-"
                                + id
                            ],
                            stdout=log,
                            stderr=subprocess.STDOUT,
                            timeout=20,
                            check=False,
                        )
                    except (OSError, subprocess.TimeoutExpired) as error:
                        meta.setdefault("collection_errors", []).append(str(error))
            meta["finished_at"] = time.time()
            save()
            self.clear()
        print(
            json.dumps({k: meta.get(k) for k in ["tier", "label", "status", "error"]}),
            flush=True,
        )
        return meta


if __name__ == "__main__":
    p = argparse.ArgumentParser()
    p.add_argument("--connection", type=pathlib.Path, required=True)
    p.add_argument("--out", type=pathlib.Path, required=True)
    p.add_argument("--games", default=",".join(GAMES))
    p.add_argument("--smoke", action="store_true")
    p.add_argument("--network", action="store_true")
    p.add_argument("--seconds", type=int, default=60)
    a = p.parse_args()
    a.out = a.out.resolve()
    repo = pathlib.Path(__file__).resolve().parents[2]
    bench = Bench(json.loads(a.connection.read_text()), a.out, repo)
    bench.clear()
    try:
        if a.network:
            for game in a.games.split(","):
                bots = 4 if game not in ["q2repro", "bzflag"] else 0
                for clients in [1, 4, 8] if game == "q2repro" else [1, 4]:
                    bench.scenario(
                        [(game, bots)],
                        f"{game}-hybrid-{clients}clients",
                        seconds=max(a.seconds, 90),
                        network_clients=clients,
                    )
                if game == "bzflag":
                    for robots in [7, 15]:
                        bench.scenario(
                            [(game, 0)],
                            f"bzflag-{robots}robots",
                            seconds=max(a.seconds, 90),
                            network_clients=1,
                            robots=robots,
                        )
            raise SystemExit(0)
        for game in a.games.split(","):
            # Q2 has no implemented engine bot AI; BZFlag robots run in external clients.
            counts = [0] if game in ["q2repro", "bzflag"] else [0, 4, 8, 16]
            if a.smoke:
                counts = counts[:2]
            for bots in counts:
                bench.scenario(
                    [(game, bots)], f"{game}-1x-{bots}bots", seconds=a.seconds
                )
            if not a.smoke and game not in ["q2repro", "bzflag"]:
                for replicas in [2, 4, 8, 12, 16]:
                    result = bench.scenario(
                        [(game, 8)] * replicas,
                        f"{game}-{replicas}x-8bots",
                        seconds=a.seconds,
                    )
                    if result["status"] == "failed":
                        break
        if not a.smoke and set(a.games.split(",")) == set(GAMES):
            for loads, label in [
                ([("fteqw", 8), ("ioquake3", 8)], "mix-quake-q3"),
                ([("ioquake3", 8), ("ut99", 8)], "mix-q3-ut"),
                (
                    [("fteqw", 8), ("ioquake3", 8), ("openarena", 8), ("ut99", 8)],
                    "mix-four",
                ),
                ([("xonotic", 8), ("ioquake3", 8)], "mix-xon-q3"),
            ]:
                bench.scenario(loads, label, seconds=a.seconds)
    finally:
        bench.clear()
