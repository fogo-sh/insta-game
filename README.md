# insta-game

Run independent dedicated game servers from one Go executable. The controller,
CLI, embedded web UI, Discord integration, and game agent share one instance
model. Choose native Linux processes (`local`), local containers (`docker`), or
AWS Fargate (`aws`). Fly is not implemented.

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
| `aws` | Stop the Fargate task; keep its instance stack and EFS data | Launch a new task attempt using the same EFS access point | Stop the task, verify scoped data cleanup, then remove the instance stack |

Mutations request a desired state; check `show` or the UI for completion and
errors. Deletion permanently removes instance data. A failed AWS cleanup remains
visible and can be retried with `delete`. Use the current connection address
after a restart, especially when AWS assigns a new task IP.

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
`/opt` game root on first startup. AWS mounts the instance's EFS directory there,
so engine files, downloaded assets, mods, configuration, and writable game files
share the same persistent root. Completed seeding is recorded and existing files
are preserved. Changing an image does not upgrade an existing seeded root.

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
             "-game", "id1", "-port", "{port}", "+exec", "server.cfg"]
  }
}
```

```sh
bin/insta-game serve --native-config ./native.json --default-provider local
bin/insta-game create --provider local --game fteqw --name "Quake night"
```

`command` and `assets` must be absolute; `configPath` is relative to the copied
assets. Arguments support `{data}`, `{config}`, `{port}`, and `{port:27960}` for
another catalog port. Configure every engine port that could collide. Native
game bind addresses are controlled by engine arguments/configuration;
`--game-bind` controls Docker port publishing. Native supervision requires Linux.

## AWS

AWS setup uses the Go binary and its embedded CloudFormation template. Configure
AWS credentials for the operator account, choose a dedicated stack name, and
replace the example CIDR below with the controller's public IPv4 `/32`. The
controller runs on your machine or another host you manage; bootstrap does not
deploy it. It must remain online and retain its database.

```sh
bin/insta-game aws plan --region ca-central-1 --stack insta-game-dev \
  --controller-cidr 203.0.113.4/32
```

Review the printed resource changes and change-set ARN, then apply that exact
change set:

```sh
bin/insta-game aws apply --region ca-central-1 --stack insta-game-dev \
  --change-set REVIEWED_CHANGE_SET_ARN --out ./aws.json
# Recover the environment config later if needed:
bin/insta-game aws config --region ca-central-1 --stack insta-game-dev --out ./aws.json
```

Bootstrap creates a VPC/public subnet, security groups, ECS cluster, EFS storage,
private asset bucket, ECR repository, IAM execution role, and log group. Game
ports are public. Agent management uses pinned TLS plus bearer authentication
and is restricted to the configured controller CIDR. Update the environment
through another plan/apply if that address changes.

Publish a built game image to the environment's registry:

```sh
bin/insta-game aws push --config ./aws.json \
  --image ghcr.io/fogo-sh/insta-game:xonotic --tag xonotic
```

`push` uses the local Docker CLI and prints the ECR image URI. In a copy of the
game catalog, set that game's `image` to this URI and set `architecture` to the
image's ECS architecture (`ARM64` for the supplied builds). Keep its default
config file beside `game.json`. Then run:

```sh
bin/insta-game serve --catalog ./aws-catalog --aws-config ./aws.json \
  --default-provider aws
bin/insta-game create --provider aws --game xonotic --name "AWS Friday" --id aws-friday
```

For licensed game data, upload your archive and use the printed private S3 URL
as the game's asset environment variable before starting the controller:

```sh
bin/insta-game aws upload --config ./aws.json --file ./quake-assets.zip --key quake-assets.zip
export FTEQW_DATA_URL=s3://YOUR_ASSETS_BUCKET/quake-assets.zip
```

The task role reads assets from this environment's bucket. Equivalent variables
are `Q2REPRO_DATA_URL`, `IOQUAKE3_DATA_URL`, and `UT99_DATA_URL`. Asset URLs are
captured when AWS resources are created. ZIP files normally extract into `/opt`;
use versioned archives with the engine's expected directory layout. Docker can
use HTTP(S) asset URLs passed through these same variables. Native instances use
the configured local asset directory.

Stopping AWS games retains EFS data, uploaded assets, images, and logs. Deleting
an instance also leaves shared environment resources. These retained resources
can continue to incur charges; stopping tasks is not environment teardown.
Delete all instances and wait for completion before destroying their environment:

```sh
bin/insta-game delete aws-friday
bin/insta-game show aws-friday
# After every instance is deleted:
bin/insta-game aws destroy --region ca-central-1 --stack insta-game-dev
# If uploaded assets remain and you intend to remove them too:
bin/insta-game aws destroy --region ca-central-1 --stack insta-game-dev --delete-assets
```

Destroy refuses an environment with remaining game tasks/stacks or a nonempty
asset bucket unless `--delete-assets` is supplied. Successful destruction removes
the shared filesystem, logs, registry images, and environment resources. Export
anything you intend to keep first. Existing deployments are not adopted or
retired automatically by this migration.

## Controller access and Discord

HTTP listens on loopback by default. For Docker games reachable on a LAN:

```sh
bin/insta-game serve --listen 0.0.0.0:3000 \
  --game-bind 0.0.0.0 --public-host 192.168.1.20
```

Put HTTPS in front of the controller for remote access. Its token grants full
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
make lint-templates  # Embedded environment and every generated game template
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
