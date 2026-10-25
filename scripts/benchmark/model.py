"""Export provisional CPU/RAM requests and measured capacity bounds from DuckDB."""

import argparse
import collections
import html
import json
import math
import pathlib

import duckdb
from history import render as render_history

p = argparse.ArgumentParser()
p.add_argument("database", type=pathlib.Path)
a = p.parse_args()
db = duckdb.connect(str(a.database), read_only=True)


def rows(sql):
    q = db.execute(sql)
    keys = [c[0] for c in q.description]
    return [dict(zip(keys, r)) for r in q.fetchall()]


runs = rows("SELECT * FROM run_summary")
hosts = rows("SELECT * FROM hosts")
costs = {}
bounds = []
for r in runs:
    r["detail"] = json.loads(r["metadata"])
    r["games"] = list(dict.fromkeys(g for g, n in r["detail"]["loads"]))
for game in sorted(set(g for r in runs for g in r["games"])):
    profile = [
        r
        for r in runs
        if r["games"] == [game]
        and r["server_count"] == 1
        and r["bots_per_server"] + r["network_clients"] == 8
        and r["status"] == "measured"
        and r["population_ok"]
        and r["input_isolated"]
        and r["host_n"] >= 20
        and r["server_cpu_p95"] is not None
    ]
    if profile:
        costs[game] = {
            "cpu_millicores": math.ceil(
                max(r["server_cpu_p95"] for r in profile) * 1000
            ),
            "memory_mib": math.ceil(
                max(r["server_memory_p95"] for r in profile) / 1048576
            ),
            "max_real_clients_tested": max(r["network_clients"] for r in profile),
            "runs": [r["run_id"] for r in profile],
        }
for h in hosts:
    for game in sorted(costs):
        groups = {}
        for r in runs:
            if (
                r["tier"] != h["tier"]
                or r["games"] != [game]
                or r["bots_per_server"] + r["network_clients"] / r["server_count"] <= 0
            ):
                continue
            key = (r["bots_per_server"], r["network_clients"] / r["server_count"])
            groups.setdefault(key, []).append(r)
        for (bots, network), group in groups.items():
            failures = [
                r
                for r in group
                if not r["passes"]
                and (
                    (
                        r["status"] == "measured"
                        and r["population_ok"]
                        and r["input_isolated"]
                        and r["host_n"] >= 20
                    )
                    or (
                        "memory safety guard" in (r["error"] or "")
                        and r["created_count"] > 0
                    )
                )
            ]
            passing = [
                r
                for r in group
                if r["passes"]
                and not any(f["server_count"] == r["server_count"] for f in failures)
            ]
            if not passing:
                continue
            count = max(r["server_count"] for r in passing)
            higher = [r["server_count"] for r in failures if r["server_count"] > count]
            bounds.append(
                {
                    "tier": h["tier"],
                    "game": game,
                    "engine_bots_per_server": bots,
                    "network_players_per_server": network,
                    "largest_consistently_passing_count": count,
                    "next_failing_count": min(higher) if higher else None,
                    "upper_boundary_reached": bool(higher),
                    "longest_passing_seconds": max(
                        r["measured_until"] - r["measured_at"]
                        for r in passing
                        if r["server_count"] == count
                    ),
                }
            )
output = {
    "status": "provisional",
    "architecture": "AWS Graviton2 ARM64",
    "region": "ca-central-1",
    "policy": {
        "cpu_headroom_fraction": 0.25,
        "memory_headroom_fraction": 0.2,
        "max_query_loss": 0.01,
        "max_query_p95_ms": 150,
    },
    "limits": [
        "Map-specific synthetic workloads; not a guarantee for all humans, maps, or mods.",
        "Engine bots and actual network players are separate profiles.",
        "CPU/RAM requests use worst single-server p95 at eight participants; resource addition is a model.",
        "T4g Unlimited surplus credits and retained storage add cost.",
    ],
    "game_requests": costs,
    "host_budgets": [
        {
            **h,
            "cpu_millicores": math.floor(1500 - (h["idle_cpu_cores"] or 0) * 1000),
            "memory_mib": math.floor(
                (h["memory_usable_mib"] or h["memory_gib"] * 1024) * 0.8
                - (h["idle_memory_mib"] or 0)
            ),
            "credit_neutral_cpu_millicores": math.floor(
                (h["baseline_cores"] - (h["idle_cpu_cores"] or 0)) * 1000
            ),
        }
        for h in hosts
    ],
    "observed_bounds": bounds,
}
mixed = [r for r in runs if len(r["games"]) > 1]
output["mixed_runs"] = []
for r in mixed:
    host = next(h for h in hosts if h["tier"] == r["tier"])
    loads = r["detail"]["loads"]
    prediction = None
    if not r["network_clients"] and all(b == 8 and g in costs for g, b in loads):
        prediction = {
            "cpu_cores": (host["idle_cpu_cores"] or 0)
            + sum(costs[g]["cpu_millicores"] for g, b in loads) / 1000,
            "memory_mib": (host["idle_memory_mib"] or 0)
            + sum(costs[g]["memory_mib"] for g, b in loads),
        }
    output["mixed_runs"].append(
        {
            **{
                k: r[k]
                for k in [
                    "run_id",
                    "tier",
                    "status",
                    "error",
                    "passes",
                    "cpu_mean",
                    "cpu_p95",
                    "memory_free_p05",
                    "latency_p95",
                    "loss",
                ]
            },
            "loads": loads,
            "network_clients": r["network_clients"],
            "network_targets": r["detail"].get("network_targets", {}),
            "host_memory_used_p95_mib": (
                (1 - r["memory_free_p05"]) * host["memory_usable_mib"]
                if r["memory_free_p05"] is not None and host["memory_usable_mib"]
                else None
            ),
            "additive_host_prediction": prediction,
        }
    )
path = a.database.with_name("capacity-model.json")
path.write_text(json.dumps(output, indent=2))
print(path)


names = {
    "fteqw": "Quake I",
    "ioquake3": "Quake III",
    "openarena": "OpenArena",
    "ut99": "UT99",
    "xonotic": "Xonotic",
}
bot_rows = []
for game, name in names.items():
    cells = []
    for tier in ["t4g.micro", "t4g.small", "t4g.medium"]:
        bound = next(
            (
                b
                for b in bounds
                if b["tier"] == tier
                and b["game"] == game
                and b["engine_bots_per_server"] == 8
                and b["network_players_per_server"] == 0
            ),
            None,
        )
        cells.append(
            ("≥" if not bound["upper_boundary_reached"] else "")
            + str(bound["largest_consistently_passing_count"])
            if bound
            else "—"
        )
    bot_rows.append(
        "<tr><th>"
        + name
        + "</th>"
        + "".join("<td>" + cell + "</td>" for cell in cells)
        + "</tr>"
    )

network_rows = []
q2 = [
    b
    for b in bounds
    if b["game"] == "q2repro"
    and b["engine_bots_per_server"] == 0
    and b["network_players_per_server"] > 0
]
if q2:
    layouts = sorted(
        {
            (b["largest_consistently_passing_count"], b["network_players_per_server"])
            for b in q2
        }
    )
    network_rows.append(
        "<li><strong>Quake II:</strong> "
        + ", ".join(f"{n} × {int(players)} clients" for n, players in layouts)
        + " passed on t4g.small; the ceiling was not reached.</li>"
    )
real_eight = [
    names[g]
    for g in ["ioquake3", "openarena", "ut99"]
    if any(
        r["games"] == [g]
        and r["passes"]
        and r["server_count"] == 1
        and r["network_clients"] == 8
        and r["bots_per_server"] == 0
        for r in runs
    )
]
if real_eight:
    network_rows.append(
        "<li><strong>"
        + ", ".join(real_eight)
        + ":</strong> eight real clients per game passed in single-server tests. Full human-lobby density was not established.</li>"
    )
bz = [
    r
    for r in runs
    if r["games"] == ["bzflag"] and r["passes"] and r["server_count"] == 1
]
if bz:
    network_rows.append(
        f"<li><strong>BZFlag:</strong> {max(r['network_clients'] for r in bz)} network players on one server passed, including client-side robots. Multi-server probe latency was inconsistent, so its packing ceiling remains unresolved.</li>"
    )
if "xonotic" in costs and not any(
    r["games"] == ["xonotic"] and r["passes"] and r["network_clients"] > 0 for r in runs
):
    network_rows.append(
        "<li><strong>Xonotic:</strong> bot capacity measured; real-client result inconclusive.</li>"
    )

margin = ""
if any(r["label"] == "soak-frik-6x8-10min" and r["passes"] for r in runs):
    margin = "<strong>For a micro, choose six Quake servers rather than eight:</strong> six left about 31% RAM free in the soak; eight only about 20%."
mixed_section = ""
if mixed:
    latest_host = max(mixed, key=lambda r: r["started_at"])["tier"]
    latest_type = next(h["instance_type"] for h in hosts if h["tier"] == latest_host)
    mixed_rows = []
    for r in sorted(mixed, key=lambda r: r["started_at"]):
        if r["tier"] != latest_host:
            continue
        counts = collections.Counter(g for g, b in r["detail"]["loads"])
        mix = ", ".join(f"{n} {names.get(g, g)}" for g, n in counts.items())
        if r["network_clients"]:
            mix += f" + {r['network_clients']} clients"
            target_games = {
                names.get(i["game"]["id"], i["game"]["id"])
                for i in r["detail"].get("instances", [])
                if i["id"] in r["detail"].get("network_targets", {})
            }
            if target_games:
                mix += " on " + ", ".join(sorted(target_games))
        duration = (
            f"{(r['measured_until'] - r['measured_at']) / 60:.1f} min"
            if r["measured_until"] and r["measured_at"]
            else "—"
        )
        cpu = f"{r['cpu_p95']:.2f}" if r["cpu_p95"] is not None else "—"
        ram = f"{r['memory_free_p05']:.0%}" if r["memory_free_p05"] is not None else "—"
        result = "Incomplete"
        if r["passes"]:
            result = "Passed"
        elif r["status"] == "measured":
            result = "Outside screen"
        mixed_rows.append(
            f"<tr><td>{html.escape(mix)}</td><td>{duration}</td><td>{cpu}</td><td>{ram}</td><td>{result}</td></tr>"
        )
    mixed_section = (
        "<h2>Mixed-game tests</h2><p>Latest host: "
        + html.escape(latest_type)
        + ". Eight bots per server; extra network clients are noted. CPU is host p95; free RAM is p05. Select a test in the timeline for query health or an incomplete run’s reason.</p>"
        + '<div class="table"><table><thead><tr><th>Mix</th><th>Measured</th><th>CPU cores</th><th>Free RAM</th><th>Result</th></tr></thead><tbody>'
        + "".join(mixed_rows)
        + "</tbody></table></div>"
    )
page = pathlib.Path(__file__).with_name("summary.html").read_text()
for key, value in {
    "__BOT_ROWS__": "".join(bot_rows),
    "__NETWORK_ROWS__": "".join(network_rows),
    "__QUAKE_MARGIN__": margin,
    "__RUN_COUNT__": str(len(runs)),
    "__HISTORY__": render_history(db, runs, hosts, output["policy"], a.database.parent),
    "__MIXED__": mixed_section,
}.items():
    page = page.replace(key, value)
a.database.with_name("index.html").write_text(page)
