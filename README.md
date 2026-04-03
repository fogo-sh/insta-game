# insta-game

`insta-game` is an on-demand game server system for AWS. A public Lambda Function URL starts and stops an ECS Fargate game server, so the service can sit at zero tasks when idle and only run when players need it.

## Repository Layout

- `pulumi/`: AWS infrastructure in Python, managed with `uv`
- `lambda/launcher/`: public Lambda handler for start, stop, and status
- `docker-containers/xonotic/`: Xonotic server image (ARM64), built from source via the Xonotic git repo
- `docker-containers/qssm/`: QSS-M Quake server image, sidecar service, and local build scripts

## Local Workflow

From `docker-containers/xonotic/`:

```sh
make download
make clean
make build
make run
```

Useful extras:

```sh
make all   # download + clean + build
make ruff  # format + lint pulumi/, lambda/, and sidecar-service/
```

The Xonotic image is ARM64-only and built from source. To build locally for ARM64:

```sh
docker buildx build --platform linux/arm64 -t xonotic:latest .
```

From `docker-containers/qssm/`:

```sh
docker compose run --rm --build \
  -e DATA_URL=https://example.com/quake-assets.zip \
  -p 26000:26000/udp \
  -p 5001:5001/tcp \
  qssm
```

The Quake asset zip should contain `id1/pak0.pak`, `id1/pak1.pak`, and an
optional `id1/server.cfg`.

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
uv run pulumi config set qssmDataUrl <quake-data-url>
```

`xonoticDataUrl` and `qssmDataUrl` override `defaultDataUrl` for those services.
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

## Public API

After deployment, get the function URL with:

```sh
uv run pulumi stack output prod_url
```

Examples:

```sh
curl "<prod_url>?game=xonotic&operation=start"
curl "<prod_url>?game=qssm&operation=start"
curl "<prod_url>?game=xonotic"
curl "<prod_url>?game=xonotic&operation=stop"
```
