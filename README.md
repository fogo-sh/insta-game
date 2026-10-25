# insta-game

Run dedicated game servers with one Go binary. Use native Linux processes,
Docker, or an AWS fleet. Manage instances through the web UI, CLI, or
Discord. Each instance has its own settings, ports, credentials, and saved data.

## Start with Docker

You need Go 1.27.3, Docker, and an ARM64 host (or emulation) for the supplied
game images. From the repository root:

```sh
make build
bin/insta-game serve
```

Open http://localhost:3000 and enter the token from `.insta-game/token`.
The UI is embedded and needs no frontend build. Missing images pull on first
use. Create a server there, or use
another terminal:

```sh
bin/insta-game create --provider docker --game xonotic --name "Friday games" --id friday
bin/insta-game list
bin/insta-game stop friday
bin/insta-game start friday
```

Stopping keeps game data. Deleting removes it. Games stop after 10 empty minutes
or 120 minutes by default; `--idle 0` and `--max-runtime 0` disable those limits.

## Games

| Game | Catalog ID | Gateway protocol | Transport |
| --- | --- | --- | --- |
| Quake 1 / FTEQW | `fteqw` | `quake1` | UDP |
| Quake 2 / q2repro | `q2repro` | `quake2` | UDP |
| Quake 3 Arena / ioquake3 | `ioquake3` | `quake3` | UDP |
| OpenArena | `openarena` | `quake3` | UDP |
| Xonotic | `xonotic` | `xonotic` | UDP |
| Unreal Tournament 99 | `ut99` | `ut99` | UDP |
| BZFlag | `bzflag` | `bzflag` | TCP and UDP on the same port |


Supply commercial game assets yourself. Native games need an installed engine
and a profile; Docker images contain the runtime. See [running servers](docs/running.md)
for native setup, AWS, stable wake addresses, storage, access, and Discord.

AWS has an always-accessible Lambda page. Friends choose any deployed game and
start a named server. Hosts fetch games on demand, share capacity, and scale up
to your host limit. Empty hosts sleep; stopping keeps data. See
[the AWS launch page](docs/running.md#aws-launch-page) and its
[screenshot](docs/images/aws-launch-page.png).

The gateway keeps clients connected while the saved game and optional AWS host
start. Quake and Xonotic show loading messages; Quake II/III and OpenArena show
connection progress. UT99 keeps its connecting screen alive, and BZFlag shows
server messages. Clients continue into the game on the same connection. See [validation](docs/validation.md)
for tested engines and remaining gameplay checks. Fly is not implemented.

## Downloads

The latest successful [main CI run](https://github.com/fogo-sh/insta-game/actions/workflows/ci.yml?query=branch%3Amain)
contains an `insta-game-packages` artifact: Linux AMD64, Linux ARM64, and macOS
ARM64 archives with checksums, catalog, and docs. Unpack one and run commands
from that directory. Go is only needed to build from source; install Docker,
game engines, and assets separately. Native supervision requires Linux; macOS
can use Docker or control a remote host.

## Development

```sh
make build
make dist        # binaries, catalog, docs, and checksums
make test        # race tests and UI tests
make check       # CloudFormation lint, vet, and Compose freshness
make images      # build prepared game images
```

`make check` needs `uv` for pinned cfn-lint; UI tests need Node 18+. Neither is
an application runtime dependency. `make lint-templates` runs template checks
without AWS credentials. Regenerate `compose.yml` with `python3 generate-compose.py`
after editing game metadata. Compose builds images; the controller runs instances.

[Architecture and API](docs/architecture.md) · [Running servers](docs/running.md) · [Validation](docs/validation.md)
