# insta-game

`insta-game` is an on-demand game server system for AWS. A public Lambda Function URL starts and stops an ECS Fargate game server, so the service can sit at zero tasks when idle and only run when players need it.

## Repository Layout

- `pulumi/`: AWS infrastructure in Python, managed with `uv`
- `lambda/launcher/`: public Lambda handler for start, stop, and status
- `docker-containers/xonotic/`: Xonotic server image, sidecar service, and local build scripts

## Local Xonotic Workflow

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
uv run pulumi config set defaultConfigUrl <server-cfg-url>
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
curl "<prod_url>?game=xonotic&operation=start&config_url=https%3A%2F%2Fexample.com%2Fserver.cfg"
curl "<prod_url>?game=xonotic"
curl "<prod_url>?game=xonotic&operation=stop"
```

The default Xonotic config is URL-based via `defaultConfigUrl`, and `config_url` can override it for a single launch.
