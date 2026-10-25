# Capacity measurements

These are development tools, not a deployment service. Use isolated, owned hosts;
keep AWS provisioning and cleanup in the existing CLI. Never aim load at a public
server without permission.

`sample.py` streams Linux and Docker cgroup counters on the host. `probe.py`
queries owned game endpoints from the load generator. Measurements need verified
player counts: starting a process does not prove that bots are playing.

The report distinguishes engine bots, real network clients, and idle servers.
`Bench.scenario` accepts a list of `(game, bot_count)` loads for mixed-game tests.
Network clients target the first server unless `spread_clients=True`; saved
target counts let the report verify that clients joined the intended servers.
Query response latency is a responsiveness proxy, not a measurement of every
simulation frame. Short burst tests do not establish sustainable T4g capacity.

Development dependencies for analysis are `duckdb==2.0.0` and `plotly==7.1.0`.
The generated HTML bundles Plotly and its data; it needs no CDN or analytics.

```sh
python3 scripts/benchmark/sample.py 2 > samples.jsonl  # on the owned host, as root
python3 scripts/benchmark/run.py --connection connection.json --out ./results --smoke
uv run --no-project --with duckdb==2.0.0 --with plotly==7.1.0 \
  python scripts/benchmark/report.py --data ./measurements --out ./report
```

The connection file points to a pinned SSH tunnel, not an unauthenticated public
controller. It names the isolated `insta-game-capacity-*` stack ARN in `stack`, tier, instance ID, local tunnel URL, and private token
file, plus the path to its live telemetry file in `samples`. Separate repeat hosts
use a distinct tier directory and set `instance_type` to their actual EC2 type. Raw input layout is
`TIER/connection.json`, `TIER/samples.jsonl`, and
`results/RUN/{run.json,probes.jsonl}`. Keep tokens and asset files out of reports.

`--network` launches real game clients using the locally installed validation
fixtures. Each gets its own X display; synthetic movement/fire never targets the
user's desktop. Client binaries/assets are not downloaded or distributed by
these scripts. BZFlag robots run in its client. Quake II's shipped server has no
implemented bot movement API, so its load must be supplied by network clients.

Generate the short summary page and provisional CPU/RAM requests after building
the database (the full report is in `explorer.html`):

```sh
uv run --no-project --with duckdb==2.0.0 \
  python scripts/benchmark/model.py ./report/capacity.duckdb
```

The summary includes an interactive D3 test timeline and linked CPU, RAM, server
count, and load-generator charts. Select a test or drag the time strip to zoom.
D3 is vendored with its license; the report works without a CDN.

Requests use observed single-server p95 resources at eight participants. The
report separates bot and network profiles, retains failed runs, and checks
population and client isolation before using a run for packing. CPU credit
neutrality is modeled separately from burst capacity. No result changes the
controller’s placement policy automatically.

The client driver uses the validation fixtures installed for this experiment
under `~/.local/share/insta-game/fixtures` (override with
`INSTA_GAME_FIXTURES`). Keep game assets and caches on disk, not a RAM-backed
`/tmp`. Its explicit
launch commands document the binaries and assets needed to reproduce a client;
install or update those paths before another run. Fixtures and commercial game
data are not part of the repository or report.

Optional `annotations.json` records analysis exclusions without changing raw run
files. DuckDB metadata preserves the original status and the reason.
