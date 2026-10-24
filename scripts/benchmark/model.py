"""Export provisional CPU/RAM requests and measured capacity bounds from DuckDB."""

import argparse
import json
import math
import pathlib

import duckdb

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
path = a.database.with_name("capacity-model.json")
path.write_text(json.dumps(output, indent=2))
print(path)
