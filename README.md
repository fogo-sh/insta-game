# insta-game

`insta-game` is an on-demand game server system for AWS. A public Lambda Function URL starts and stops an ECS Fargate game server, so the service can sit at zero tasks when idle and only run when players need it.

## Repository Layout

- `pulumi/`: AWS infrastructure in Python, managed with `uv`
- `lambda/launcher/`: public Lambda handler for start, stop, and status
- `docker-containers/xonotic/`: Xonotic server image (x86_64), sidecar service, and local build scripts
- `docker-containers/xonotic-arm/`: Xonotic server image (ARM64), built from source via the Xonotic git repo

## Local Workflow

From `docker-containers/xonotic/` or `docker-containers/xonotic-arm/`:

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

For the ARM image, build with:

```sh
docker buildx build --platform linux/arm64 -t xonotic-arm:latest .
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
```

`defaultDataUrl` is passed to all game services as `DATA_URL`. It accepts one or more `url=path` pairs separated by `;`. Each entry is downloaded at container startup — zip files are extracted to the given path, and raw files are written directly to the given path. This is the mechanism for supplying game data and server config without baking it into the image.

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
curl "<prod_url>?game=xonotic-arm&operation=start"
curl "<prod_url>?game=xonotic"
curl "<prod_url>?game=xonotic&operation=stop"
```
