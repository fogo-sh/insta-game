"""Build an offline interactive report and a queryable DuckDB from benchmark JSON."""

import argparse
import json
import pathlib

import duckdb
from plotly.offline import get_plotlyjs

p = argparse.ArgumentParser()
p.add_argument("--data", type=pathlib.Path, required=True)
p.add_argument("--out", type=pathlib.Path, required=True)
a = p.parse_args()
a.out.mkdir(parents=True, exist_ok=True)
db = duckdb.connect(str(a.out / "capacity.duckdb"))
db.execute("BEGIN")
for table in [
    "runs",
    "host_samples",
    "container_samples",
    "probes",
    "hosts",
    "generator_samples",
    "cloudwatch",
    "runtime_overrides",
]:
    db.execute("DROP TABLE IF EXISTS " + table)
db.execute(
    "CREATE TABLE hosts(tier VARCHAR, instance_id VARCHAR, memory_gib DOUBLE, vcpus INTEGER, baseline_cores DOUBLE, hourly_usd DOUBLE,idle_cpu_cores DOUBLE,idle_memory_mib DOUBLE,memory_usable_mib DOUBLE,instance_type VARCHAR)"
)
specs = {
    "t4g.micro": (1, 0.2, 0.0092),
    "t4g.small": (2, 0.4, 0.0184),
    "t4g.medium": (4, 0.4, 0.0368),
}
for path in sorted(a.data.glob("t4g.*/connection.json")):
    connection = json.loads(path.read_text())
    tier = path.parent.name
    instance_type = connection.get("instance_type", tier)
    mem, baseline, price = specs[instance_type]
    db.execute(
        "INSERT INTO hosts VALUES (?,?,?,?,?,?,?,?,?,?)",
        [
            tier,
            connection["instance_id"],
            mem,
            2,
            baseline,
            price,
            None,
            None,
            None,
            instance_type,
        ],
    )
db.execute(
    "CREATE TABLE runs(run_id VARCHAR PRIMARY KEY,tier VARCHAR,label VARCHAR,load_kind VARCHAR,server_count INTEGER,bots_per_server DOUBLE,network_clients INTEGER,status VARCHAR,error VARCHAR,started_at DOUBLE,measured_at DOUBLE,measured_until DOUBLE,metadata JSON,population_ok BOOLEAN,created_count INTEGER,input_isolated BOOLEAN)"
)
runs = []
annotations = (
    json.loads((a.data / "annotations.json").read_text())
    if (a.data / "annotations.json").exists()
    else {}
)
for path in sorted((a.data / "results").glob("*/run.json")):
    r = json.loads(path.read_text())
    r["client_configurations"] = {
        file.parent.name: json.loads(file.read_text())
        for file in path.parent.glob("client-*/client-info.json")
    }
    if r["run_id"] in annotations:
        r["annotation"] = annotations[r["run_id"]]
        if "analysis_status" in r["annotation"]:
            r["recorded_status"] = r["status"]
            r["status"] = r["annotation"]["analysis_status"]
    runs.append(r)
    loads = r["loads"]
    db.execute(
        "INSERT INTO runs VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
        [
            r["run_id"],
            r["tier"],
            r["label"],
            r.get("load_kind", "unknown"),
            len(loads),
            sum(x[1] for x in loads) / len(loads),
            r.get("network_clients", 0),
            r["status"],
            r.get("error"),
            r["started_at"],
            r.get("measured_at"),
            r.get("measured_until"),
            json.dumps(r),
            not r.get("network_clients", 0)
            or (
                bool(r.get("final_players"))
                and min((value or 0) for value in r["final_players"].values())
                >= r["network_clients"] / len(loads)
            ),
            len(list(path.parent.glob("cap-*.log"))),
            not (r.get("network_clients", 0) > 1 and loads[0][0] == "q2repro")
            or (
                len(list(path.parent.glob("client-*/client-info.json")))
                >= r["network_clients"]
                and all(
                    json.loads(file.read_text()).get("isolated_wine_prefix")
                    and json.loads(file.read_text()).get("explicit_qport")
                    for file in path.parent.glob("client-*/client-info.json")
                )
            ),
        ],
    )
db.execute(
    "CREATE TABLE host_samples(tier VARCHAR,time DOUBLE,cpu_cores DOUBLE,steal_cores DOUBLE,memory_total BIGINT,memory_available BIGINT,swap_used BIGINT,load1 DOUBLE,rx_bytes BIGINT,tx_bytes BIGINT,active_containers INTEGER,oom_kills BIGINT,major_faults BIGINT)"
)
db.execute(
    "CREATE TABLE container_samples(tier VARCHAR,time DOUBLE,container_id VARCHAR,server_id VARCHAR,cpu_cores DOUBLE,memory_bytes BIGINT,working_set_bytes BIGINT,oom_kill BIGINT,throttled_usec BIGINT,network_rx_bytes BIGINT,network_tx_bytes BIGINT)"
)
for path in a.data.glob("t4g.*/samples.jsonl"):
    previous = None
    hostrows = []
    containerrows = []
    for line in path.read_text().splitlines():
        try:
            r = json.loads(line)
        except json.JSONDecodeError:
            continue
        cpu = steal = None
        if previous:
            delta = [v - o for v, o in zip(r["cpu_ticks"], previous["cpu_ticks"])]
            dt = r["time"] - previous["time"]
            total = sum(delta[:8])
            busy = total - delta[3] - delta[4]
            if total > 0:
                cpu = 2 * busy / total
                steal = 2 * delta[7] / total
        hostrows.append(
            [
                path.parent.name,
                r["time"],
                cpu,
                steal,
                r["memory_total"],
                r["memory_available"],
                r["swap_used"],
                r["load1"],
                r.get("rx_bytes", 0),
                r.get("tx_bytes", 0),
                len(r["containers"]),
                r.get("oom_kills", 0),
                r.get("major_faults", 0),
            ]
        )
        for c in r["containers"]:
            containerrows.append(
                [
                    path.parent.name,
                    r["time"],
                    c["id"],
                    c.get("name", "").removeprefix("insta-game-"),
                    c.get("cpu_cores"),
                    c["memory_bytes"],
                    c["working_set_bytes"],
                    c["oom_kill"],
                    c["throttled_usec"],
                    c.get("network_rx_bytes"),
                    c.get("network_tx_bytes"),
                ]
            )
        previous = r
    if hostrows:
        db.executemany(
            "INSERT INTO host_samples VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)", hostrows
        )
    if containerrows:
        db.executemany(
            "INSERT INTO container_samples VALUES (?,?,?,?,?,?,?,?,?,?,?)",
            containerrows,
        )
db.execute(
    "CREATE TABLE probes(run_id VARCHAR,time DOUBLE,server_id VARCHAR,game VARCHAR,ok BOOLEAN,latency_ms DOUBLE,players INTEGER,bytes BIGINT,error VARCHAR)"
)
for path in (a.data / "results").glob("*/probes.jsonl"):
    rows = []
    for line in path.read_text().splitlines():
        r = json.loads(line)
        rows.append(
            [
                r[k]
                for k in [
                    "run_id",
                    "time",
                    "server_id",
                    "game",
                    "ok",
                    "latency_ms",
                    "players",
                    "bytes",
                    "error",
                ]
            ]
        )
    if rows:
        db.executemany("INSERT INTO probes VALUES (?,?,?,?,?,?,?,?,?)", rows)
db.execute(
    "CREATE TABLE generator_samples(run_id VARCHAR,client VARCHAR,time DOUBLE,memory_available BIGINT,load1 DOUBLE,logical_cpus INTEGER,client_cpu_ticks BIGINT,client_rss_bytes BIGINT,cpu_ticks JSON,memory_full_avg10 DOUBLE)"
)
generator_paths = list((a.data / "results").glob("*/client-*/generator.jsonl"))
if (a.data / "generator-samples.jsonl").exists():
    generator_paths.append(a.data / "generator-samples.jsonl")
for path in generator_paths:
    values = []
    for line in path.read_text().splitlines():
        r = json.loads(line)
        values.append(
            [
                path.parent.parent.name if path.name == "generator.jsonl" else None,
                path.parent.name if path.name == "generator.jsonl" else "aggregate",
                r["time"],
                r["memory_available"],
                r["load1"],
                r.get("logical_cpus"),
                r.get("client_cpu_ticks"),
                r.get("client_rss_bytes"),
                json.dumps(r["cpu_ticks"]),
                r.get("memory_full_avg10"),
            ]
        )
    if values:
        db.executemany(
            "INSERT INTO generator_samples VALUES (?,?,?,?,?,?,?,?,?,?)", values
        )
db.execute(
    "CREATE TABLE cloudwatch(tier VARCHAR,metric VARCHAR,time TIMESTAMPTZ,value DOUBLE,status VARCHAR)"
)
if (a.data / "cloudwatch.json").exists():
    for metric in json.loads((a.data / "cloudwatch.json").read_text())[
        "MetricDataResults"
    ]:
        tier, name = metric["Label"].split("/", 1)
        values = [
            [tier, name, t, v, metric["StatusCode"]]
            for t, v in zip(metric["Timestamps"], metric["Values"])
        ]
        if values:
            db.executemany("INSERT INTO cloudwatch VALUES (?,?,?,?,?)", values)
db.execute(
    "CREATE TABLE runtime_overrides(time DOUBLE,server_id VARCHAR,command VARCHAR)"
)
if (a.data / "runtime-overrides.jsonl").exists():
    for line in (a.data / "runtime-overrides.jsonl").read_text().splitlines():
        r = json.loads(line)
        db.execute(
            "INSERT INTO runtime_overrides VALUES (?,?,?)",
            [r["time"], r["server_id"], r["command"]],
        )
db.execute("COMMIT")
db.execute(
    "UPDATE hosts SET idle_cpu_cores=(SELECT quantile_cont(cpu_cores,.5) FROM host_samples WHERE host_samples.tier=hosts.tier AND active_containers=0),idle_memory_mib=(SELECT quantile_cont(memory_total-memory_available,.95)/1048576.0 FROM host_samples WHERE host_samples.tier=hosts.tier AND active_containers=0),memory_usable_mib=(SELECT max(memory_total)/1048576.0 FROM host_samples WHERE host_samples.tier=hosts.tier)"
)
sql = """
CREATE OR REPLACE VIEW run_summary AS
WITH h AS (
 SELECT r.run_id, count(*) host_n, avg(s.cpu_cores) cpu_mean, quantile_cont(s.cpu_cores,.95) cpu_p95,
 min(s.memory_available)*1.0/max(s.memory_total) memory_free_min, quantile_cont(s.memory_available,.05)/max(s.memory_total) memory_free_p05,
 max(s.oom_kills)-min(s.oom_kills) host_oom_kills,max(s.major_faults)-min(s.major_faults) major_faults,max(s.steal_cores) steal_max, max(s.swap_used) swap_max,
 (max(s.tx_bytes)-min(s.tx_bytes))/nullif(max(s.time)-min(s.time),0) tx_bytes_sec
 FROM runs r JOIN host_samples s ON r.tier=s.tier AND s.time BETWEEN r.measured_at AND r.measured_until GROUP BY r.run_id
), q AS (
 SELECT run_id,count(*) probe_n, avg(case when ok then 0.0 else 1.0 end) loss,
 quantile_cont(latency_ms,.95) latency_p95, quantile_cont(latency_ms,.99) latency_p99,
 min(players) players_min,max(players) players_max FROM probes GROUP BY run_id
), c AS (
 SELECT r.run_id,max(s.oom_kill) oom_kill,quantile_cont(s.working_set_bytes,.95) server_memory_p95,quantile_cont(s.cpu_cores,.95) server_cpu_p95
 FROM runs r JOIN container_samples s ON r.tier=s.tier AND s.time BETWEEN r.measured_at AND r.measured_until GROUP BY r.run_id
)
SELECT r.*,h.* EXCLUDE(run_id),q.* EXCLUDE(run_id),c.* EXCLUDE(run_id),hosts.baseline_cores,hosts.hourly_usd,
 r.status='measured' AND r.population_ok AND r.input_isolated AND h.host_n>=20 AND q.loss<=.01 AND q.latency_p95<=150 AND h.cpu_p95<=1.5 AND h.memory_free_p05>=.2 AND coalesce(c.oom_kill,0)=0 AND h.host_oom_kills=0 AS passes,
 greatest(0,h.cpu_mean-hosts.baseline_cores)*.04 AS modeled_surplus_usd_hour
FROM runs r LEFT JOIN h USING(run_id) LEFT JOIN q USING(run_id) LEFT JOIN c USING(run_id) JOIN hosts USING(tier);
"""
db.execute(sql)
(a.out / "analysis.sql").write_text(
    sql
    + "\n-- Largest tested passing counts, not extrapolated maxima:\nSELECT tier,label,server_count,cpu_p95,memory_free_p05,latency_p95,loss,passes FROM run_summary ORDER BY tier,label;\n"
)


def records(query):
    result = db.execute(query)
    keys = [x[0] for x in result.description]
    return [dict(zip(keys, row)) for row in result.fetchall()]


summary = records("SELECT * EXCLUDE(metadata) FROM run_summary ORDER BY started_at")
for row in summary:
    meta = next(r for r in runs if r["run_id"] == row["run_id"])
    row["games"] = list(dict.fromkeys(g for g, n in meta["loads"]))
    row["loads"] = meta["loads"]
time_series = records(
    "SELECT r.run_id,s.time,s.cpu_cores,s.memory_available/1048576.0 memory_free_mib FROM runs r JOIN host_samples s ON r.tier=s.tier AND s.time BETWEEN r.measured_at AND r.measured_until ORDER BY s.time"
)
hosts = records("SELECT * FROM hosts")
(a.out / "summary.json").write_text(json.dumps(summary, indent=2))
(a.out / "hosts.json").write_text(json.dumps(hosts, indent=2))
credit_metrics = records(
    "SELECT tier,metric,epoch(time) time,value FROM cloudwatch WHERE metric IN ('CPUCreditBalance','CPUSurplusCreditBalance','CPUSurplusCreditsCharged') ORDER BY time"
)
data = json.dumps(
    {"runs": summary, "samples": time_series, "hosts": hosts, "credits": credit_metrics}
).replace("</", "<\\/")
html = pathlib.Path(__file__).with_name("report.html").read_text()
experiment = a.data / "experiment.json"
if experiment.exists():
    (a.out / "experiment.json").write_text(experiment.read_text())
html = html.replace(
    "__EXPERIMENT_LINK__",
    ' · <a href="experiment.json" download>Experiment / cleanup</a>'
    if experiment.exists()
    else "",
)
(a.out / "index.html").write_text(
    html.replace("__PLOTLY__", get_plotlyjs()).replace("__DATA__", data)
)
print(
    json.dumps(
        {
            "runs": len(runs),
            "host_samples": db.execute("SELECT count(*) FROM host_samples").fetchone()[
                0
            ],
            "probes": db.execute("SELECT count(*) FROM probes").fetchone()[0],
            "database": str(a.out / "capacity.duckdb"),
        }
    )
)
db.close()
