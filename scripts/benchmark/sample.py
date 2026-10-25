"""Read Linux/cgroup counters on an isolated benchmark host; emit JSON lines."""

import http.client
import json
import pathlib
import socket
import sys
import time

root = pathlib.Path("/sys/fs/cgroup/system.slice")
interval = float(sys.argv[1]) if len(sys.argv) > 1 else 2
previous = {}
previous_time = None
names = {}
while True:
    now = time.time()
    stat = list(
        map(int, pathlib.Path("/proc/stat").read_text().splitlines()[0].split()[1:])
    )
    mem = {
        k: int(v.split()[0]) * 1024
        for k, v in (
            line.split(":", 1)
            for line in pathlib.Path("/proc/meminfo").read_text().splitlines()
        )
    }
    net = {}
    for line in pathlib.Path("/proc/net/dev").read_text().splitlines()[2:]:
        name, values = line.split(":")
        values = values.split()
        if name.strip().startswith(("en", "eth")):
            net = {"rx_bytes": int(values[0]), "tx_bytes": int(values[8])}
    containers = []
    for scope in root.glob("docker-*.scope"):
        cid = scope.name[7:-6]
        try:
            data = dict(
                line.split() for line in (scope / "cpu.stat").read_text().splitlines()
            )
            memory = int((scope / "memory.current").read_text())
            cache = dict(
                line.split()
                for line in (scope / "memory.stat").read_text().splitlines()
            )
            events = dict(
                line.split()
                for line in (scope / "memory.events").read_text().splitlines()
            )
            cid = scope.name[7:-6]
            if cid not in names:
                connection = http.client.HTTPConnection("localhost")
                connection.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
                connection.sock.connect("/var/run/docker.sock")
                connection.request("GET", "/containers/" + cid + "/json")
                info = json.load(connection.getresponse())
                connection.close()
                if info.get("State", {}).get("Pid", 0) <= 0:
                    continue
                names[cid] = (info.get("Name", "").lstrip("/"), info["State"]["Pid"])
            if not names[cid][0].startswith("insta-game-cap-"):
                continue
            value = {
                "id": cid,
                "name": names[cid][0],
                "cpu_usec": int(data["usage_usec"]),
                "memory_bytes": memory,
                "working_set_bytes": max(
                    0, memory - int(cache.get("inactive_file", 0))
                ),
                "oom_kill": int(events["oom_kill"]),
                "throttled_usec": int(data.get("throttled_usec", 0)),
            }
            if previous_time and cid in previous:
                value["cpu_cores"] = (
                    (value["cpu_usec"] - previous[cid]) / 1e6 / (now - previous_time)
                )
            for line in (
                pathlib.Path("/proc", str(names[cid][1]), "net/dev")
                .read_text()
                .splitlines()[2:]
            ):
                interface, fields = line.split(":")
                fields = fields.split()
                if interface.strip() == "eth0":
                    value.update(
                        network_rx_bytes=int(fields[0]), network_tx_bytes=int(fields[8])
                    )
            containers.append(value)
        except (FileNotFoundError, ProcessLookupError):
            names.pop(cid, None)
            continue
    vm = dict(
        line.split() for line in pathlib.Path("/proc/vmstat").read_text().splitlines()
    )
    result = {
        "oom_kills": int(vm.get("oom_kill", 0)),
        "major_faults": int(vm.get("pgmajfault", 0)),
        "time": now,
        "cpu_ticks": stat,
        "memory_total": mem["MemTotal"],
        "memory_available": mem["MemAvailable"],
        "swap_used": mem["SwapTotal"] - mem["SwapFree"],
        "load1": float(pathlib.Path("/proc/loadavg").read_text().split()[0]),
        "containers": containers,
        **net,
    }
    print(json.dumps(result), flush=True)
    previous = {c["id"]: c["cpu_usec"] for c in containers}
    previous_time = now
    time.sleep(interval)
