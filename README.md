# insta-game

Run independent dedicated game servers from one Go executable. The controller,
CLI, embedded web UI, Discord integration, and game agent share one instance
model. Choose native Linux processes (`local`), local containers (`docker`), or
a shared AWS EC2 host running either provider. Fly is not implemented.

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
             "+set", "pr_ssqc_memsize", "67108864", "+exec", "server.cfg"]
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

AWS runs the controller and several games on one EC2 host. Game creation uses
`local` exactly as it does on your machine; `docker` also works if you install
a Docker daemon and compatible game images on the host. The host keeps its
controller database and game files on encrypted gp3 EBS. Stopping EC2 retains
that disk; destroying the stack deletes it.

```sh
ssh-keygen -t ed25519 -N '' -f ./game-host-key
bin/insta-game aws plan --stack insta-game-dev \
  --operator-cidr YOUR_PUBLIC_IP/32 --public-key ./game-host-key.pub
# Review the printed changes, then apply their exact ARN:
bin/insta-game aws apply --stack insta-game-dev --change-set REVIEWED_ARN \
  --key ./game-host-key --token-file ./game-host-token --out ./aws.json
```

Set native engine paths in `native.json` to `/opt/insta-game/engine/ENGINE_NAME`
and assets to `/opt/insta-game/assets`, then install your compatible Linux AMD64
binary, engine, licensed assets, and a catalog containing that game's metadata:

```sh
bin/insta-game aws install --config ./aws.json --binary ./bin/insta-game \
  --engine /path/to/fteqw.sv --assets /path/to/quake-assets \
  --native-config ./native.json --catalog ./game-catalog
bin/insta-game aws wake --config ./aws.json
# Use the returned IP; client flags precede instance IDs:
bin/insta-game create --url https://HOST_IP:3000 --token-file ./game-host-token \
  --provider local --game fteqw --id quake-a --name "Quake night"
bin/insta-game wake --config ./aws.json \
  --route 100.YOUR.TAILNET.IP:26200=quake-a
```

Repeat `--route ADDRESS=GAME_ID` for other saved games on different wake ports.
The listener requires AWS credentials and access to the controller's token.
`aws status --config ./aws.json` reports EC2 state. `aws stop --config ./aws.json`
stops the whole host; `aws destroy --config ./aws.json` removes the stack and all
saved game data. Export data before destruction.

The default host is a `t3a.micro` with 1 GiB RAM and an 8 GiB disk. CPU credits
use Standard mode, which can throttle sustained workloads but avoids surplus
credit charges. Size the host for the games and player count you actually run.

The Go binary provisions the host through a reviewed CloudFormation change set
and installs the controller and your native engine/assets over verified SSH.
SSH and the controller API accept traffic only from your operator CIDR. Game
UDP ports are public. The VM needs no AWS credentials.

The controller stops each empty game according to its idle policy, then powers
off the host after all games have stopped for two minutes. A separate six-hour
boot timer bounds a failed experiment even if the controller fails. Starting
EC2 assigns a fresh public address; connection information is refreshed from
the controller.

A wake listener can run on an existing machine, such as your tailnet host.
It accepts Quake connection attempts, starts EC2 and the selected saved game,
and replies with a rejection message while the server boots. Retry to obtain
`connect IP:PORT`, then connect directly. It does not relay gameplay. Status
queries do not wake the server. Keep the listener private to avoid unwanted
starts. Lambda function URLs accept HTTPS, so they cannot answer Quake UDP
connection attempts.

At Canada Central prices checked on 2026-10-22, the default EC2 host costs
US$0.0104/hour plus US$0.005/hour for its public IPv4 while running. Its 8 GiB
gp3 disk costs US$0.704/month while retained. Three games sharing the host cost
approximately **US$11.95/month continuously**, or **US$1.63/month at two hours
of total host uptime per day**. These estimates exclude transfer, taxes, and
any cost of your existing wake-listener machine. Calculate a retained month as
`$0.704 + host-running-hours × $0.0154`. Boot and idle grace periods
count as host uptime. Stopping games alone is not enough: verify EC2 reaches
`stopped`, or destroy the stack to remove the disk charge too.

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
[Go migration status](docs/go-unification.md) for validation scope.

`make check` uses `uv` to run pinned cfn-lint in an isolated Python environment.
Template linting needs no AWS credentials and adds no application runtime dependency.

The UI renderer regression tests use Node 18+ (`node --test`); Node is not
needed to build or run the Go application.
