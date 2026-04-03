# Repository Guidelines

## Project Structure & Module Organization
`pulumi/` contains AWS infrastructure code in Python. `pulumi/__main__.py` defines the stack, and `pulumi/game_service.py` holds the reusable ECS game service component. `lambda/launcher/` contains the TypeScript Lambda Function URL handler, Discord command registration script, and bundled output in `lambda/launcher/dist/`. `sidecar/` contains the Go sidecar binary source and protocol query implementations in `sidecar/protocol/`. `docker-containers/xonotic/`, `docker-containers/qssm/`, `docker-containers/q2repro/`, and `docker-containers/bzflag/` contain the game image builds and local shell scripts. There is no dedicated `tests/` directory yet; validation is mostly command-based.

## Build, Test, and Development Commands
From `pulumi/`:

```sh
uv sync
uv run ruff format .
uv run ruff check .
uv run pulumi preview
uv run pulumi up
```

Use `preview` before applying infrastructure changes.

From `sidecar/`:

```sh
make build      # cross-compile for linux/arm64
make test       # go test ./...
make vet        # go vet ./...
```

From `lambda/launcher/`:

```sh
npm install
npm run build       # bundle src/index.ts into dist/index.js
npm run register    # register Discord slash commands
```

From `docker-containers/xonotic/`, `docker-containers/qssm/`, `docker-containers/q2repro/`, or `docker-containers/bzflag/`:

```sh
make download   # fetch required game assets when applicable
make clean      # remove generated local assets when applicable
make build      # build the local Docker image
make run        # run the local container
```

## Coding Style & Naming Conventions
Use 4-space indentation in Python. Keep code straightforward and avoid unnecessary abstraction. Format and lint Python with Ruff via the `pulumi/` UV project. AWS resource names should follow the current stack convention: regional resources include the region code, and global IAM-style resources include both region and account suffix. Keep Pulumi logical resource names stable unless replacement is intended.
