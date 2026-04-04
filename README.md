# insta-game

`insta-game` is an on-demand game server system for AWS. A public Lambda Function URL starts and stops an ECS Fargate game server, so the service can sit at zero tasks when idle and only run when players need it.

## Repository Layout

- `pulumi/`: AWS infrastructure in Python, managed with `uv`
- `launcher/`: Hono app — Lambda handler (AWS) and Docker backend (self-hosted)
- `sidecar/`: Go sidecar binary — HTTP control API and process manager for game containers
- `docker-containers/xonotic/`: Xonotic server image (ARM64), built from source via the Xonotic git repo
- `docker-containers/qssm/`: QSS-M Quake 1 server image and local build scripts
- `docker-containers/q2repro/`: q2repro Quake 2 server image and local build scripts
- `docker-containers/bzflag/`: BZFlag server image and local build scripts
- `docker-containers/ut99/`: Unreal Tournament GOTY server image and local build scripts

## Local Workflow

From the repo root:

```sh
docker compose up xonotic   # run Xonotic
docker compose up qssm      # run QSS-M / Quake 1 (requires DATA_URL env var — see compose.yml)
docker compose up q2repro   # run q2repro / Quake 2 (requires DATA_URL env var — see compose.yml)
docker compose up bzflag    # run BZFlag
docker compose up ut99      # run UT99 GOTY (requires DATA_URL env var — see compose.yml)
```

All game images, including UT99 GOTY, are `linux/arm64`. Make sure Docker
Desktop has QEMU/multi-platform support enabled when your host architecture
does not match the image.

To build images locally (handles any required pre-build steps automatically):

```sh
./build.sh xonotic
./build.sh qssm
./build.sh q2repro
./build.sh bzflag
./build.sh ut99
```

For QSS-M, q2repro, and UT99, `DATA_URL` is required because commercial game assets are not bundled. In local Compose, set `QSSM_DATA_URL`, `Q2REPRO_DATA_URL`, or `UT99_DATA_URL` in `.env`; `build.sh` will prompt for and save these values automatically. Each value can contain one or more `;`-separated `url=path` entries. Each entry is either a zip (extracted to `path`) or a raw file (written to `path`). You can also supply just a URL with no `=path` and the sidecar will extract to the default game directory:

```sh
# Quake 1 — zip containing id1/pak0.pak and id1/pak1.pak
QSSM_DATA_URL="https://example.com/quake-assets.zip=/opt/" docker compose up qssm

# Quake 2 — zip containing baseq2/pak0.pak etc.
Q2REPRO_DATA_URL="https://example.com/quake2-assets.zip" docker compose up q2repro

# UT99 — zip containing SystemARM64/ucc-bin-arm64, Maps/, Textures/, Music/, Sounds/,
# and optionally data/UnrealTournament.ini
UT99_DATA_URL="https://example.com/ut99.zip" docker compose up ut99
```

Downloaded data is cached in `.cache/<game>/` and reused on subsequent runs — the sidecar skips the download if it already has a sentinel file from a previous successful fetch.

Set `RCON_PASSWORD` in your local `.env` to configure the admin password for
all game servers. `QSSM_RCON_PASSWORD` can override only QSS-M. In-game admin
login commands differ by engine:

- Xonotic, QSS-M, q2repro: `rcon_password <password>`
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

Local sidecar status:

```sh
curl http://127.0.0.1:5001/status
```

## Infrastructure Workflow

From `pulumi/`:

```sh
uv sync
uv run ruff format .
uv run ruff check .
uv run pulumi preview
uv run pulumi up
```

Set required config:

```sh
uv run pulumi config set --secret sidecarToken <token>
uv run pulumi config set defaultDataUrl <data-url>
uv run pulumi config set xonoticDataUrl <xonotic-data-url>
uv run pulumi config set --secret rconPassword <rcon-password>
uv run pulumi config set qssmDataUrl <quake1-data-url>
uv run pulumi config set q2reproDataUrl <quake2-data-url>
uv run pulumi config set bzflagDataUrl <bzflag-config-url>
uv run pulumi config set ut99DataUrl <ut99-zip-url>
uv run pulumi config set --secret webUiPassphrase <passphrase>
uv run pulumi config set --secret apiToken <token>
uv run pulumi config set --secret discordPublicKey <public-key>
uv run pulumi config set --secret discordBotToken <bot-token>
uv run pulumi config set discordAppId <app-id>
uv run pulumi config set budgetAlertEmail <email>
uv run pulumi config set monthlyBudgetLimitUsd 50
uv run pulumi config set enableCustomDomain false
```

`xonoticDataUrl`, `qssmDataUrl`, `q2reproDataUrl`, `bzflagDataUrl`, and `ut99DataUrl` override `defaultDataUrl` for those services.
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

This registers `/start`, `/stop`, and `/status` globally. Safe to re-run.

## Public API

After deployment, get the function URL with:

```sh
uv run pulumi stack output prod_url
```

Examples:

```sh
curl "<prod_url>?game=xonotic&operation=start"
curl "<prod_url>?game=qssm&operation=start"
curl "<prod_url>?game=q2repro&operation=start"
curl "<prod_url>?game=bzflag&operation=start"
curl "<prod_url>?game=ut99&operation=start"
curl "<prod_url>?game=xonotic"
curl "<prod_url>?game=xonotic&operation=stop"
```

## Self-hosted (Docker)

The launcher can run locally against Docker instead of ECS.

1. Build the Docker bundle:
   ```sh
   cd launcher
   npm run build:docker
   ```

2. Set environment variables in `.env`:
   ```
   WEB_UI_PASSPHRASE=your-passphrase
   API_TOKEN=your-api-token
   SIDECAR_TOKEN=abc123
   ```

3. Start everything:
   ```sh
   docker compose up launcher
   ```

The launcher will be available at `http://localhost:3000`. It manages the other game containers via the Docker socket.
