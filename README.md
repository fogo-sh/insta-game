# insta-game

`insta-game` is an on-demand game server system for AWS. A public Lambda Function URL starts and stops an ECS Fargate game server, so the service can sit at zero tasks when idle and only run when players need it.

## Repository Layout

- `pulumi/`: AWS infrastructure in Python, managed with `uv`
- `lambda/launcher/`: public Lambda handler for start, stop, and status
- `sidecar/`: Go sidecar binary — HTTP control API and process manager for game containers
- `docker-containers/xonotic/`: Xonotic server image (ARM64), built from source via the Xonotic git repo
- `docker-containers/qssm/`: QSS-M Quake 1 server image and local build scripts
- `docker-containers/q2repro/`: q2repro Quake 2 server image and local build scripts

## Local Workflow

From the repo root:

```sh
docker compose up xonotic   # run Xonotic
docker compose up qssm      # run QSS-M / Quake 1 (requires DATA_URL env var — see compose.yml)
docker compose up q2repro   # run q2repro / Quake 2 (requires DATA_URL env var — see compose.yml)
```

The images are ARM64-only. Make sure Docker Desktop has QEMU/multi-platform support enabled, or run on an ARM64 machine.

To build images locally (handles any required pre-build steps automatically):

```sh
./build.sh xonotic
./build.sh qssm
./build.sh q2repro
```

For QSS-M and q2repro, `DATA_URL` is required — Quake pak files are commercial and not bundled. Set it to one or more `;`-separated `url=path` entries. Each entry is either a zip (extracted to `path`) or a raw file (written to `path`). You can also supply just a URL with no `=path` and the sidecar will extract to the default game directory:

```sh
# Quake 1 — zip containing id1/pak0.pak and id1/pak1.pak
DATA_URL="https://example.com/quake-assets.zip=/opt/" docker compose up qssm

# Quake 2 — zip containing baseq2/pak0.pak etc.
DATA_URL="https://example.com/quake2-assets.zip" docker compose up q2repro
```

Downloaded data is cached in `.cache/<game>/` and reused on subsequent runs — the sidecar skips the download if it already has a sentinel file from a previous successful fetch. `build.sh` will prompt for and save the URL to `.env` automatically.

For Xonotic, `DATA_URL` is optional — the image ships with a default `server.cfg`. Set it only if you want to supply a custom config:

```sh
# override server.cfg only
DATA_URL="https://example.com/server.cfg=/opt/data/server.cfg" docker compose up xonotic
```

Local sidecar status:

```sh
curl http://127.0.0.1:5001/status
```

## Infrastructure Workflow

From `pulumi/`:

```sh
uv sync
uv run pulumi preview
uv run pulumi up
```

Set required config:

```sh
uv run pulumi config set --secret sidecarToken <token>
uv run pulumi config set defaultDataUrl <data-url>
uv run pulumi config set xonoticDataUrl <xonotic-data-url>
uv run pulumi config set qssmDataUrl <quake1-data-url>
uv run pulumi config set q2reproDataUrl <quake2-data-url>
```

`xonoticDataUrl`, `qssmDataUrl`, and `q2reproDataUrl` override `defaultDataUrl` for those services.
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

### Registering Discord slash commands

Run once after setting up the Discord application:

```sh
cd lambda/launcher
DISCORD_APP_ID=<app-id> DISCORD_BOT_TOKEN=<bot-token> npx ts-node register-commands.ts
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
curl "<prod_url>?game=xonotic"
curl "<prod_url>?game=xonotic&operation=stop"
```
