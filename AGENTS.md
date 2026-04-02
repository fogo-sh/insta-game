# Repository Guidelines

## Project Structure & Module Organization
`pulumi/` contains AWS infrastructure code in Python. `pulumi/__main__.py` defines the stack, and `pulumi/game_service.py` holds the reusable ECS game service component. `lambda/launcher/` contains the Lambda Function URL handler for start, stop, and status operations. `docker-containers/xonotic/`, `docker-containers/xonotic-arm/`, and `docker-containers/qssm/` contain the game image builds, local shell scripts, Docker Compose setup, and the Flask sidecar in `sidecar-service/`. There is no dedicated `tests/` directory yet; validation is mostly command-based.

## Build, Test, and Development Commands
From `pulumi/`:

```sh
uv sync
uv run pulumi preview
uv run pulumi up
```

Use `preview` before applying infrastructure changes.

From `docker-containers/xonotic/`, `docker-containers/xonotic-arm/`, or `docker-containers/qssm/`:

```sh
make download   # fetch Xonotic 0.8.6
make clean      # build Xonotic-clean/
make build      # build the local Docker image
make run        # run the local container
make ruff       # format and lint pulumi/, lambda/, and sidecar-service/
```

## Coding Style & Naming Conventions
Use 4-space indentation in Python. Keep code straightforward and avoid unnecessary abstraction. Format and lint Python with Ruff via the `pulumi/` UV project. AWS resource names should follow the current stack convention: regional resources include the region code, and global IAM-style resources include both region and account suffix. Keep Pulumi logical resource names stable unless replacement is intended.
