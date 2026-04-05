# insta-game

`insta-game` runs on-demand dedicated game servers in one of two modes:

- **AWS deployment**: Pulumi provisions Lambda + ECS Fargate, and the launcher starts/stops game tasks in AWS.
- **Local/self-hosted deployment**: Docker Compose runs the launcher plus game containers on one machine, and the launcher controls local containers through the Docker socket.

These modes are separate. Use the Pulumi workflow for AWS, or use `docker compose` for local/self-hosted runs. You do not need Docker Compose for the AWS deployment path.

Current AWS game services are:
- Xonotic on ARM64 Fargate
- FTEQW / Quake 1 on ARM64 Fargate with 1024 CPU and 2048 MiB memory
- q2repro / Quake 2 on ARM64 Fargate
- ioquake3 / Quake 3 Arena on ARM64 Fargate
- OpenArena on ARM64 Fargate
- BZFlag on ARM64 Fargate
- UT99 GOTY on ARM64 Fargate

## Repository Layout

- `pulumi/`: AWS infrastructure in Python, managed with `uv`
- `launcher/`: Hono app with two backends: Lambda handler for AWS, Docker API backend for self-hosted Compose
- `sidecar/`: Go sidecar binary — HTTP control API and process manager for game containers
- `docker-containers/<game>/`: one folder per local game image; each supported game folder includes a `Dockerfile` and `game.json`

## Game Metadata

For local Docker and CI image publishing, `docker-containers/<game>/game.json` is
the source of truth for a game. It defines the game ID, display name, sidecar
protocol, ports, launch command, optional volumes, and any runtime `DATA_URL`
environment variable name.

Adding a new game folder with both a `Dockerfile` and `game.json` automatically:

- includes it in local `./build.sh`
- includes it in the GHCR image publish workflow
- exposes it to the local Docker launcher backend
- adds it to Discord slash-command registration
- gives the sidecar the correct protocol without a separate `PROTOCOL` env var

This does not yet auto-create a new `docker compose` service or AWS/Pulumi ECS
service. Those remain explicit.

## Self-Hosted Docker Workflow

Use this path when you want to run the launcher and game servers locally with
Docker Compose instead of deploying AWS infrastructure.

From the repo root:

```sh
docker compose up xonotic   # run Xonotic
docker compose up fteqw     # run FTEQW / Quake 1 (requires DATA_URL env var — see compose.yml)
docker compose up q2repro   # run q2repro / Quake 2 (requires DATA_URL env var — see compose.yml)
docker compose up ioquake3  # run ioquake3 / Quake 3 Arena (requires DATA_URL env var — see compose.yml)
docker compose up openarena # run OpenArena
docker compose up bzflag    # run BZFlag
docker compose up ut99      # run UT99 GOTY (requires DATA_URL env var — see compose.yml)
```

All game images, including UT99 GOTY, are `linux/arm64`. Make sure Docker
Desktop has QEMU/multi-platform support enabled when your host architecture
does not match the image.

To build images locally (handles any required pre-build steps automatically):

```sh
./build.sh xonotic
./build.sh fteqw
./build.sh q2repro
./build.sh ioquake3
./build.sh openarena
./build.sh bzflag
./build.sh ut99
```

`./build.sh` now discovers buildable games from `docker-containers/*/game.json`
plus `Dockerfile`, so you do not need to edit the script when adding another
game folder.

For FTEQW, q2repro, ioquake3, and UT99, `DATA_URL` is required because commercial game assets are not bundled. In local Compose, set `FTEQW_DATA_URL`, `Q2REPRO_DATA_URL`, `IOQUAKE3_DATA_URL`, or `UT99_DATA_URL` in `.env`; `build.sh` will prompt for and save these values automatically. Each value can contain one or more `;`-separated `url=path` entries. Each entry is either a zip (extracted to `path`) or a raw file (written to `path`). You can also supply just a URL with no `=path` and the sidecar will extract to the default game directory:

```sh
# Quake 1 — zip containing id1/pak0.pak and id1/pak1.pak
FTEQW_DATA_URL="https://example.com/quake-assets.zip=/opt/" docker compose up fteqw

# Quake 2 — zip containing baseq2/pak0.pak etc.
Q2REPRO_DATA_URL="https://example.com/quake2-assets.zip" docker compose up q2repro

# Quake 3 Arena — zip containing baseq3/pak0.pk3 and any patch pk3 files
IOQUAKE3_DATA_URL="https://example.com/quake3-assets.zip" docker compose up ioquake3

# UT99 — zip containing SystemARM64/ucc-bin-arm64, Maps/, Textures/, Music/, Sounds/,
# and optionally data/UnrealTournament.ini
UT99_DATA_URL="https://example.com/ut99.zip" docker compose up ut99
```

Downloaded data is cached in `.cache/<game>/` and reused on subsequent runs — the sidecar skips the download if it already has a sentinel file from a previous successful fetch.

Set `RCON_PASSWORD` in your local `.env` to configure the admin password for
all game servers. In-game admin login commands differ by engine:

- Xonotic, FTEQW, q2repro: `rcon_password <password>`
- ioquake3, OpenArena: `/rconpassword <password>` then `\rcon <command>` from a client
- BZFlag: `/password <password>`
- UT99: `adminlogin <password>`

For Xonotic and BZFlag, `DATA_URL` is optional because each image ships with a default config. Set it only if you want to supply a custom config:

```sh
# override server.cfg only
DATA_URL="https://example.com/server.cfg=/opt/data/server.cfg" docker compose up xonotic

# override BZFlag server.cfg only
DATA_URL="https://example.com/server.cfg=/opt/data/server.cfg" docker compose up bzflag
```

For UT99 GOTY, provide a zip at runtime through `UT99_DATA_URL`. The archive
must extract into `/opt` and include `SystemARM64/ucc-bin-arm64` plus the game
content directories (`Maps/`, `Textures/`, `Music/`, `Sounds/`). Optionally
include `data/UnrealTournament.ini` to override the bundled default config:

```sh
UT99_DATA_URL="https://example.com/ut99.zip" docker compose up ut99
```

To run the local launcher UI/API against those Docker-managed game containers:

```sh
cd launcher
npm install
npm run build:docker
cd ..
docker compose up launcher
```

Set `WEB_UI_PASSPHRASE`, `API_TOKEN`, and `SIDECAR_TOKEN` in `.env` if you do
not want the default local values from `compose.yml`.

The launcher will be available at `http://localhost:3000`. It manages the game
containers through the mounted Docker socket. In Docker mode it discovers games
from `docker-containers/*/game.json`, not from a hard-coded list in
`compose.yml`.

### Adding a new local game

To make a new game available to local builds, CI image publishing, and the
Docker launcher backend:

1. Create `docker-containers/<game>/Dockerfile`.
2. Create `docker-containers/<game>/game.json`.
3. Optionally add `Makefile`, `download.sh`, and `clean.sh` if the image needs
   build-context preparation.

Minimal example:

```json
{
  "id": "mygame",
  "displayName": "My Game",
  "protocol": "quake2",
  "sidecarPort": 5001,
  "gamePort": 27910,
  "ports": {
    "27910/udp": { "hostPort": "27910" },
    "5001/tcp": { "hostPort": "5010" }
  },
  "gameCmd": "./mygame-server",
  "gameArgs": "+set dedicated 1",
  "gameQuitCmd": "quit",
  "gameQuitTimeout": 15
}
```

If the game needs runtime-downloaded assets, set `dataUrlEnv`, for example
`"dataUrlEnv": "MYGAME_DATA_URL"`, and provide that environment variable when
running the launcher or the container.

Local sidecar status:

```sh
curl http://127.0.0.1:5001/status
```

## AWS Infrastructure Workflow

Use this path when you want Lambda + ECS Fargate on AWS. This workflow does not
use `docker compose`.

From `pulumi/`:

```sh
uv sync
uv run ruff format .
uv run ruff check .
uv run pulumi preview
uv run pulumi up
```

Set required stack config:

```sh
# Launcher auth
uv run pulumi config set --secret webUiPassphrase <passphrase>
uv run pulumi config set --secret apiToken <token>

# In-game admin auth
uv run pulumi config set --secret rconPassword <rcon-password>

# Discord integration
uv run pulumi config set --secret discordPublicKey <public-key>
uv run pulumi config set --secret discordBotToken <bot-token>
uv run pulumi config set discordAppId <app-id>

# Billing alerts
uv run pulumi config set budgetAlertEmail <email>
```

Optional stack config:

```sh
# Shared fallback asset/config URL for every game service
uv run pulumi config set defaultDataUrl <data-url>

# Per-game DATA_URL overrides
uv run pulumi config set xonoticDataUrl <xonotic-data-url>
uv run pulumi config set fteqwDataUrl <quake1-data-url>
uv run pulumi config set q2reproDataUrl <quake2-data-url>
uv run pulumi config set ioquake3DataUrl <quake3-data-url>
uv run pulumi config set bzflagDataUrl <bzflag-config-url>
uv run pulumi config set ut99DataUrl <ut99-zip-url>

# Network and budget defaults
uv run pulumi config set cidrBlock 172.16.0.0/16
uv run pulumi config set monthlyBudgetLimitUsd 50

# Custom domain
uv run pulumi config set customDomainHostname <games-hostname>
uv run pulumi config set enableCustomDomain false
```

`xonoticDataUrl`, `fteqwDataUrl`, `q2reproDataUrl`, `ioquake3DataUrl`, `bzflagDataUrl`, and `ut99DataUrl` override `defaultDataUrl` for those services.
Each value is passed to the container as `DATA_URL` and accepts one or more
`url=path` pairs separated by `;`. Each entry is downloaded at container
startup, zip files are extracted to the given path, and raw files are written
directly to the given path. This is the mechanism for supplying game data and
server config without baking it into the image.

Example:

```
https://example.com/data.zip=/opt/;https://example.com/server.cfg=/opt/data/server.cfg
```

Authenticate with standard AWS environment variables, for example:

```sh
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_REGION=ca-central-1
```

If you use temporary credentials, also set `AWS_SESSION_TOKEN`.

`monthlyBudgetLimitUsd` creates AWS Budget email alerts at 50%, 80%, and 100%
forecasted spend.

`cidrBlock` defaults to `172.16.0.0/16`.

`customDomainHostname` is only required when `enableCustomDomain=true`.

### Custom domain

CloudFront cannot attach `customDomainHostname` until the ACM certificate is
issued, so the first deploy should leave `enableCustomDomain=false`.

```sh
cd pulumi
uv run pulumi config set customDomainHostname <games-hostname>
uv run pulumi up --yes
uv run pulumi stack output cert_validation_cname
```

Create the exported ACM validation CNAME in your DNS provider, wait for the
certificate to become `ISSUED`, then enable the alias and deploy again:

```sh
uv run pulumi config set enableCustomDomain true
uv run pulumi up --yes
uv run pulumi stack output games_url
```

Add a DNS CNAME for `customDomainHostname` pointing at `games_url` with
Cloudflare proxying disabled. If `enableCustomDomain` is still `false`,
`customDomainHostname` returns a CloudFront `403` because that hostname is not
attached to the distribution yet.

### Registering Discord slash commands

Run once after setting up the Discord application:

```sh
cd launcher
```

Build the Lambda bundle before `pulumi preview` or `pulumi up`:

```sh
npm install
npm run build
```

Register Discord slash commands:

```sh
DISCORD_APP_ID=<app-id> DISCORD_BOT_TOKEN=<bot-token> npm run register
```

This registers `/start`, `/stop`, and `/status` globally. In local development,
the game choices are generated from `docker-containers/*/game.json`. Safe to
re-run.

## Public API

After deployment, get the function URL with:

```sh
uv run pulumi stack output prod_url
```

Examples:

```sh
# Web UI routes (HTML responses, passphrase auth for start/stop/status actions)
curl "<prod_url>/"
curl -H "x-passphrase: <passphrase>" "<prod_url>/?game=xonotic&operation=status"
curl -H "x-passphrase: <passphrase>" "<prod_url>/?game=xonotic&operation=start"
curl -H "x-passphrase: <passphrase>" "<prod_url>/?game=xonotic&operation=stop"

# JSON API routes
curl -H "x-api-token: <api-token>" "<prod_url>/api?game=xonotic"
curl -H "x-api-token: <api-token>" "<prod_url>/api?game=fteqw&operation=start"
curl -H "x-api-token: <api-token>" "<prod_url>/api?game=q2repro&operation=stop"

# Log stream proxy for a running game (SSE)
curl -N "<prod_url>/logs?game=xonotic&token=<passphrase>"

# Same operations work for each game key
curl -H "x-api-token: <api-token>" "<prod_url>/api?game=bzflag&operation=start"
curl -H "x-api-token: <api-token>" "<prod_url>/api?game=ut99&operation=start"
```
