"""Build chart data from recorded runs and telemetry, including failed attempts."""

import json
import pathlib


def render(db, runs, hosts, policy, directory):
    def query(sql):
        return db.execute(sql).fetchall()

    last_probe = dict(query("SELECT run_id,max(time) FROM probes GROUP BY run_id"))
    timeline = []
    for r in runs:
        meta = r["detail"]
        annotation = meta.get("annotation", {})
        if r["passes"]:
            outcome = "pass"
        elif (
            r["status"] in ("probe-validation", "setup-error", "interrupted")
            or not r["input_isolated"]
            or annotation.get("kind")
            in ("operator-stopped-generator", "generator-memory-pressure")
        ):
            outcome = "check"
        elif "memory safety guard" in (r["error"] or "") and r["created_count"] > 0:
            outcome = "pressure"
        elif r["status"] == "measured":
            outcome = "pressure"
        else:
            outcome = "fail"
        finish = meta.get("finished_at") or r["measured_until"]
        reason = annotation.get("reason") or r["error"]
        if not reason and outcome == "pressure":
            flags = []
            if r["memory_free_p05"] is not None and r["memory_free_p05"] < 0.2:
                flags.append("less than 20% free RAM")
            if r["cpu_p95"] is not None and r["cpu_p95"] > 1.5:
                flags.append("CPU above 1.5 cores")
            if r["loss"] is not None and r["loss"] > 0.01:
                flags.append("query loss above 1%")
            if r["latency_p95"] is not None and r["latency_p95"] > 150:
                flags.append("query p95 above 150 ms")
            reason = (
                "Outside screening: " + ", ".join(flags)
                if flags
                else "Outside the screening policy."
            )
        timeline.append(
            {
                "id": r["run_id"],
                "host": r["tier"],
                "label": r["label"],
                "games": r["games"],
                "servers": r["server_count"],
                "bots": r["bots_per_server"],
                "clients": r["network_clients"],
                "start": r["started_at"],
                "end": finish or last_probe.get(r["run_id"]) or r["started_at"],
                "end_known": bool(finish),
                "m0": r["measured_at"],
                "m1": r["measured_until"],
                "outcome": outcome,
                "reason": reason,
                "cpu": r["cpu_p95"],
                "free": r["memory_free_p05"],
                "latency": r["latency_p95"],
                "loss": r["loss"],
            }
        )
    experiment = directory / "experiment.json"
    cleanup = (
        json.loads(experiment.read_text()).get("local_cleanup", {})
        if experiment.exists()
        else {}
    )
    payload = {
        "runs": sorted(timeline, key=lambda r: r["start"]),
        "hosts": hosts,
        "policy": policy,
        "samples": query(
            "SELECT tier,time,cpu_cores,memory_available*1.0/memory_total,active_containers FROM host_samples ORDER BY tier,time"
        ),
        "generator": query(
            "SELECT min(time),min(memory_available)/1073741824.0,max(memory_full_avg10) FROM generator_samples GROUP BY floor(time/2) ORDER BY 1"
        ),
        "cleanup": cleanup.get("time"),
    }
    source = pathlib.Path(__file__).parent
    markup = (source / "history.html").read_text()
    for key, value in {
        "__D3__": (source / "vendor/d3.min.js").read_text(),
        "__HISTORY_DATA__": json.dumps(payload, separators=(",", ":")).replace(
            "</", "<\\/"
        ),
        "__HISTORY_JS__": (source / "history.js").read_text(),
    }.items():
        markup = markup.replace(key, value)
    return markup
