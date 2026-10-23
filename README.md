# insta-game

Run independent dedicated game servers from one Go executable. The controller,
CLI, embedded web UI, Discord integration, and game agent share one instance
model. Choose native Linux processes (`local`), local containers (`docker`), or
a shared AWS EC2 host running either provider. Fly is not implemented.

## Games

| Game | Catalog ID | Gateway protocol | Transport |
| --- | --- | --- | --- |
| Quake 1 / FTEQW | `fteqw` | `quake1` | UDP, loading messages before sign-on |
| Quake 2 / q2repro | `q2repro` | `quake2` | UDP |
| Quake 3 Arena / ioquake3 | `ioquake3` | `quake3` | UDP |
| OpenArena | `openarena` | `quake3` | UDP |
| Xonotic | `xonotic` | `xonotic` | UDP |
| Unreal Tournament 99 | `ut99` | `ut99` | UDP |
| BZFlag | `bzflag` | `bzflag` | TCP and UDP on the same port |

Each game can have independent instances, configuration, ports, and storage.
Docker images supply the engine runtime; native games need an operator-installed
engine profile. Commercial game assets must be supplied separately. Runtime validation coverage is recorded in
[validation](docs/validation.md).

## Binary bundles

CI's `insta-game-packages` artifact contains Linux AMD64, Linux ARM64, and macOS
ARM64 archives plus `SHA256SUMS`. Each archive includes `bin/insta-game`, the
catalog, and these docs. Unpack your target archive and run commands from that
directory; Go is needed only when building from source. Game engines, Docker,
and commercial assets are installed separately.

Build the same bundles locally with `make dist`. Native supervision runs on
Linux; the macOS binary can use Docker or administer a remote controller.

## Start with Docker

Requires Go 1.26, Docker Compose, and game images matching the runtime
architecture. The supplied game image builds target Linux ARM64; use an ARM64
host or suitable emulation for local Docker.

From the repository root:

```sh
make build
make -C docker-containers/xonotic download
make -C docker-containers/xonotic clean
docker compose --profile images build xonotic
bin/insta-game serve
```

Open **http://localhost:3000** and enter the token from `.insta-game/token`.
The simple UI lists servers, creates instances with game settings, shows their
current addresses and status, and provides logs, console commands, stop/start,
and deletion. It is embedded in the binary; there is no frontend build step.

```sh
bin/insta-game games
bin/insta-game create --provider docker --game xonotic --name "Friday games" --id friday
bin/insta-game create --provider docker --game xonotic --name "Sunday CTF" --id sunday --config ./ctf.cfg
bin/insta-game list
bin/insta-game show friday
bin/insta-game logs friday
bin/insta-game console friday status
bin/insta-game stop friday
bin/insta-game start friday
bin/insta-game delete friday
```

Client flags, including `--url` and `--token-file`, go before positional IDs.
`INSTA_GAME_TOKEN` overrides the token file. The controller loads game metadata
and default configs from `docker-containers`; use `serve --catalog PATH` for a
separate catalog. Docker images must already be built or pulled.

## Lifecycle and storage

Creation saves the selected provider, configuration, credentials, and desired
state before allocating resources. A stable `--id` makes creation retries
idempotent. Changing the default provider affects new instances only.

| Provider | Stop | Start again | Delete |
| --- | --- | --- | --- |
| `local` | Stop the game; keep its agent and private directory | Restart the game with the saved configuration | Shut down the agent and remove the owned directory |
| `docker` | Stop the container; keep its writable layer | Start the same container | Remove the owned container and its data |

Mutations request a desired state; check `show` or the UI for completion and
errors. Deletion permanently removes instance data. Use the current connection
address after a host restart; AWS may assign a different public IP.

The CLI and UI default to 10 minutes of confirmed emptiness and 120 minutes of
maximum runtime. `--idle 0` and `--max-runtime 0` disable those limits. Query
failures mean unknown activity. Bots are excluded where the protocol identifies
them. A game crash requires an explicit stop/start; it is not an unlimited
restart loop. Keep the controller running to enforce timeout policy.

`--data` selects the controller directory, containing the private bbolt database,
token, and native instance files. Back up this directory and the game data you
need. Run one controller per database. Stopping the controller leaves game
resources running; restarting it with the same data and provider configuration
resumes reconciliation.

Game images keep a seed copy outside `/opt` and populate each instance's full
`/opt` game root on first startup. Docker retains it in the container's writable
layer. Completed seeding is recorded and existing files are preserved. Changing
an image does not upgrade an existing seeded root.

## Native Linux games

Install the engine and assets yourself, then define their paths and arguments in
`native.json`. Assets are copied into each instance; directories and regular
files are accepted, while symlinks are rejected.

```json
{
  "fteqw": {
    "command": "/srv/games/fteqw.sv",
    "assets": "/srv/games/quake-assets",
    "configPath": "id1/server.cfg",
    "args": ["-dedicated", "8", "-nohome", "-basedir", "{data}",
             "-game", "id1", "-port", "{port}",
             "+set", "pr_ssqc_memsize", "67108864",
             "+set", "sv_listen_nq", "1", "+exec", "server.cfg"]
  }
}
```

```sh
bin/insta-game serve --native-config ./native.json --default-provider local
bin/insta-game create --provider local --game fteqw --name "Quake night"
```

The FTE example caps the QuakeC VM at 64 MiB; its default 2 GiB allocation can fail
on small hosts. Larger mods may need a higher value.

`command` and `assets` must be absolute; `configPath` is relative to the copied
assets. Arguments support `{data}`, `{config}`, `{port}`, and `{port:27960}` for
another catalog port. Configure every engine port that could collide. Native
game bind addresses are controlled by engine arguments/configuration;
`--game-bind` controls Docker port publishing. Native supervision requires Linux.

## AWS

One stoppable EC2 host runs the same controller and providers. Choose CPU, memory, and disk for the number of games and players you run.
Encrypted gp3 retains the database and game files while EC2 is stopped.
Destroying the stack deletes that disk and all saved games.

The supplied Docker images target ARM64. For those images, provision a `t4g`
host and build an ARM64 controller. Native AMD64 deployments can use `t3` or
`t3a` instead; the installer checks the controller binary against the host.

```sh
ssh-keygen -t ed25519 -N '' -f ./game-host-key
bin/insta-game aws plan --stack insta-game-host --instance-type t4g.small \
  --disk-gib 32 --operator-cidr YOUR_PUBLIC_IP/32 \
  --game-cidr YOUR_GATEWAY_PUBLIC_IP/32 --public-key ./game-host-key.pub
# Review the changes and apply their exact ARN:
bin/insta-game aws apply --stack insta-game-host --change-set REVIEWED_ARN \
  --key ./game-host-key --token-file ./game-host-token --out ./aws.json

CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/insta-game-linux-arm64 ./cmd/insta-game
bin/insta-game aws install --config ./aws.json \
  --binary ./bin/insta-game-linux-arm64 --catalog ./docker-containers --pull-images
bin/insta-game aws wake --config ./aws.json
```

Installation enables Docker and uploads the controller plus catalog metadata and
configs. `--pull-images` pulls the images named by that catalog; omit it when
images are already installed. Use images built from the same release as the
controller. `--env-file ./host.env` installs a private systemd environment file
for asset URLs such as `FTEQW_DATA_URL`, `Q2REPRO_DATA_URL`, `IOQUAKE3_DATA_URL`,
and `UT99_DATA_URL`, or Discord settings. Keep those URLs and credentials out of
the catalog. Limit a custom catalog to the games you want installed. An 8 GiB
root disk can be too small for several games and their assets.

Native engines are optional. Upload a directory containing any number of engines
and runtime files with `--engines`, an asset directory with `--assets`, and
profiles with `--native-config`. Profile paths on the host use
`/opt/insta-game/engine` and `/opt/insta-game/assets`; preserve per-game
subdirectories. Stop affected games before updating their engines or assets.
Select `--provider local` when creating a native instance. Docker is the host's
default provider.

Use the IP returned by `aws wake` to create saved instances through the ordinary
controller API. All catalog games use the same creation and lifecycle commands:

```sh
bin/insta-game create --url https://HOST_IP:3000 --token-file ./game-host-token \
  --provider docker --game openarena --id arena --name "Arena night"
bin/insta-game aws status --config ./aws.json
bin/insta-game aws stop --config ./aws.json
# Permanent deletion of the host and its saved games:
bin/insta-game aws destroy --config ./aws.json
```

SSH and the controller API accept only `--operator-cidr`. Game TCP and UDP use
`--game-cidr`: restrict it to your gateway public IP when proxying, or to player
networks for direct hosting. Its default is `0.0.0.0/0`. The VM needs no AWS
credentials. The operator binary uses reviewed CloudFormation changes, verified
SSH installation, and token-pinned controller TLS.

The controller stops empty games according to their idle policies, then powers
off the host after all games have stopped for two minutes. The host also has a
boot lifetime cap (six hours by default). Set `aws install --host-max-runtime 0`
to disable that cap, or give a duration such as `12h`. Starting EC2 can change its public IP; the gateway
resolves fresh game endpoints after startup. Standard CPU credits avoid surplus
credit charges but can throttle sustained workloads.

A retained host costs its disk price plus running hours, public IPv4 hours, and
transfer. The number of games changes the required capacity, not the number of
VMs. Include startup and idle grace periods when estimating running hours;
verify EC2 reaches `stopped` to end compute and public IPv4 charges.

## Stable game addresses

Run the gateway on an existing machine reachable by players. It starts a saved
game through its controller; `--config` additionally wakes an AWS host. Without
AWS, use `--url` and `--token-file` for a native or Docker controller:

```sh
bin/insta-game wake --config ./aws.json \
  --route quake3://100.YOUR.TAILNET.IP:27960=arena

bin/insta-game wake --url http://127.0.0.1:3000 --token-file .insta-game/token \
  --route xonotic://100.YOUR.TAILNET.IP:26000=friday \
  --route bzflag://100.YOUR.TAILNET.IP:5154=tanks
```

Repeat `--route PROTOCOL://ADDRESS=INSTANCE_ID` for other saved games. Protocols
are listed in the game table. Omitting the protocol selects `quake1`. An optional
`/CATALOG_PORT` after the instance ID selects a specific backend endpoint.
The listener requires the controller token and, for AWS, operator credentials.
Keep listeners on private/tailnet addresses.

Players always use the gateway address. NetQuake connections receive loading
messages while the backend starts, then enter the game without reconnecting.
Other protocols retain their own handshakes and packet numbers. The first UDP
handshake is retained during startup; if a client stops waiting, retry the same
address.

Each player gets a separate backend socket. BZFlag's TCP greeting and UDP link
negotiation use the same front port. Its initial greeting is shared by joins and
queries, so BZFlag TCP queries can wake the host. Quake-family browser queries
and UT99 GameSpy queries do not wake it; the gateway does not provide a full
server-browser service or proxy UT99's additional query ports. Use direct
hosting when those services are required.

The FTE backend must use `sv_listen_nq 1` for NetQuake loading; the supplied config
and native example set it. Reliable packet sequence translation accounts for
the loading messages; unreliable packets remain unchanged. QuakeWorld is not
supported by this loading path.

A gateway process permits 64 TCP/UDP sessions; a BZFlag player normally uses
one of each. Startup is limited to three minutes, failed starts have a one-minute
cooldown, and inactive UDP sessions expire after one minute. The gateway carries gameplay traffic, so its location and bandwidth affect latency
and capacity. It needs a persistent TCP/UDP listener; a Lambda function URL
cannot receive game UDP packets.

## Controller access and Discord

HTTP listens on loopback by default. For Docker games reachable on a LAN:

```sh
bin/insta-game serve --listen 0.0.0.0:3000 \
  --game-bind 0.0.0.0 --public-host 192.168.1.20
```

Use `serve --tls` with the CLI's pinned TLS client, or put HTTPS in front of
the controller for browser access. Its token grants full
administration. The Docker provider requires a local daemon and reaches agents
through host loopback. A containerized controller therefore needs Linux host
networking, the Docker socket, and persistent storage:

```sh
docker build -t insta-game .
docker run --rm --network host \
  -v /var/run/docker.sock:/var/run/docker.sock -v insta-game-control:/data \
  insta-game serve --listen 127.0.0.1:3000 --data /data --catalog /opt/games
```

Discord uses the same instance API. Set `DISCORD_PUBLIC_KEY` and
`DISCORD_GUILD_ID` for `serve`, and point the application's interaction endpoint
at the controller's public HTTPS `/discord` route. Preview command definitions
with `bin/insta-game discord commands`. Register them with
`bin/insta-game discord register` using `DISCORD_APP_ID`, `DISCORD_GUILD_ID`, and
`DISCORD_BOT_TOKEN`. This replaces that guild's application command list with
`/create`, `/start`, `/stop`, and `/status`; start/stop take instance IDs.

## Development

```sh
make build
make dist        # Linux AMD64/ARM64 and macOS ARM64 bundles with catalog and checksums
make test        # Go race tests and UI renderer tests
make check       # Template lint, go vet, and Compose freshness check
make lint-templates  # Embedded shared-host CloudFormation template
make images      # build all prepared game images
```

`compose.yml` contains image-build targets; the Go controller creates running
servers. Regenerate it with `python3 generate-compose.py` after metadata changes.
An opt-in Docker test exercises two real Xonotic instances:

```sh
INSTA_GAME_DOCKER_CATALOG=/path/to/catalog go test -v ./internal/control \
  -run TestDockerTwoServersSurviveControllerRestart -count=1 -timeout=4m
```

See [architecture](docs/architecture.md) for API and provider details, and
[Validation scope](docs/validation.md) for validation scope.

`make check` uses `uv` to run pinned cfn-lint in an isolated Python environment.
Template linting needs no AWS credentials and adds no application runtime dependency.

The UI renderer regression tests use Node 18+ (`node --test`); Node is not
needed to build or run the Go application.
