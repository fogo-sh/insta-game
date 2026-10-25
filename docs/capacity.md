# Fleet capacity

Reservations decide placement. They do not limit a container's CPU, RAM, or
writable files. Raise a game's reservation before increasing players, bots, or
mod complexity. Stop retains the reservation because its data stays on that host;
Remove releases it.

These defaults use the October 24–25, 2026 ARM64 tests: engine bots, graphical
clients with movement/fire input, and mixed games. The table shows the highest
per-server p95 among passing single-game runs. Workloads included eight network
clients for Quake II/III, OpenArena, and UT99; Quake had mixed bots/clients, BZFlag
had client-side robots, and Xonotic's sizing came from eight engine bots.
[The saved measurements](capacity-evidence.json) include the aggregation rule.
These are starting estimates, not tested ceilings. Quake I network sizing
remains provisional after correcting its client launcher's command-line length
limit; the fleet acceptance run verifies joins, not a new capacity ceiling.

| Game | Observed CPU, millicores | Reserved CPU | Observed RAM, MiB | Reserved RAM | Retained disk, MiB |
| --- | ---: | ---: | ---: | ---: | ---: |
| Quake I | 40.57 | 60 | 44.36 | 96 | 256 |
| Quake II | 27.73 | 40 | 49.12 | 160 | 512 |
| Quake III | 110.74 | 140 | 80.61 | 192 | 1024 |
| OpenArena | 112.74 | 150 | 79.86 | 256 | 1024 |
| UT99 | 244.91 | 320 | 71.00 | 256 | 1024 |
| Xonotic | 92.26 | 150 | 448.20 | 576 | 2048 |
| BZFlag | 9.15 | 20 | 25.18 | 128 | 128 |

CPU reservations round up beyond 25% headroom over the observed p95. RAM includes
at least that margin; disk values allow retained game data and logs. The default
32 GiB disk leaves 12 GiB outside server reservations for the OS, cached images,
shared assets, and preparation. Large custom asset packs need a different budget.

A small host reserves 350 millicores and 1400 MiB for games; a medium keeps the
same CPU budget and raises RAM to 3200 MiB. A micro uses 150 millicores and 640 MiB.
Small and medium have the same 400-millicore aggregate baseline on these T-family
profiles; more RAM does not buy more sustained CPU. Standard credit mode avoids
surplus credit charges and can throttle bursts after credits run out.
[AWS documents the baseline rates](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/burstable-credits-baseline-concepts.html).

Override individual entries with `aws lobby --fleet-config`:

```json
{
  "games": [
    {"id": "fteqw", "name": "Quake I", "resources": {"cpu": 100, "memoryMiB": 128, "diskMiB": 512}}
  ],
  "idleMinutes": 15,
  "maxMinutes": 360
}
```

Other catalog entries keep their defaults. A reservation must fit one host.
Allocation uses its saved values even after a release changes the defaults.
Compatible UI updates reuse hosts; engine/data changes use new host slots.
Existing servers keep their pinned release.
A full host limit is an admission refusal, never permission to move saved data
or provision a larger, more expensive machine.
