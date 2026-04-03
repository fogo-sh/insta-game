# Design: Go Sidecar

**Date:** 2026-04-02

## Goal

Replace the two duplicated Python sidecar services with a single statically-compiled Go binary. This eliminates the Python runtime and uv toolchain from both game images, reduces image size, simplifies Dockerfiles, and makes ARM64 cross-compilation trivial.

## Source Layout

```
sidecar/
  go.mod
  main.go          # startup, signal handling, HTTP server, auto-shutdown loop, process lifecycle
  download.go      # DATA_URL fetch + extract logic (with retry + User-Agent)
  protocol/
    xonotic.go     # DarkPlaces UDP getinfo query → ServerInfo
    quake1.go      # NetQuake CCREQ_SERVER_INFO query → ServerInfo
```

`protocol.ServerInfo` is a shared struct:
```go
type ServerInfo struct {
    Players  int
    Hostname string
    Map      string
}
```

Each protocol file exports a single function: `Query(port int) (*ServerInfo, error)`. `main.go` selects the implementation at startup based on the `PROTOCOL` env var.

## Configuration (env vars)

All game-specific behaviour is driven by env vars. No game-specific code paths in the binary.

| Var | Xonotic value | QSSM value | Notes |
|---|---|---|---|
| `PROTOCOL` | `xonotic` | `quake1` | selects query implementation |
| `GAME_CMD` | `./xonotic-linux-arm64-dedicated` | `./qssm` | binary path to launch |
| `GAME_ARGS` | _(empty)_ | `-dedicated 12 -basedir /opt -game id1 -port 26000 +exec server.cfg` | space-separated args passed to the game process |
| `GAME_QUIT_CMD` | `exit` | `quit` | string written to game stdin to request graceful shutdown |
| `GAME_QUIT_TIMEOUT` | `30` | `15` | seconds to wait for graceful exit before SIGTERM |
| `GAME_PORT` | `26000` | `26000` | UDP port to send status queries to |
| `DATA_URL` | _(deploy-time)_ | _(deploy-time)_ | semicolon-separated `url=path` pairs, unchanged from Python |
| `DEFAULT_CONFIG_PATH` | `/opt/default-server.cfg` | `/opt/default-server.cfg` | copied to game dir if no DATA_URL set |
| `DOWNLOAD_USER_AGENT` | _(Go default)_ | _(browser UA)_ | HTTP User-Agent for data downloads |
| `TOKEN` | _(deploy-time)_ | _(deploy-time)_ | Bearer token for `/restart` and `/stop` |
| `IDLE_TIMEOUT_SECONDS` | `600` | `600` | seconds idle before ECS self-shutdown; 0 to disable |
| `ECS_CLUSTER` | _(deploy-time)_ | _(deploy-time)_ | ECS cluster name |
| `ECS_SERVICE` | _(deploy-time)_ | _(deploy-time)_ | ECS service name |
| `AWS_REGION` | `ca-central-1` | `ca-central-1` | AWS region for boto3/SDK calls |
| `PORT` | `5001` | `5001` | HTTP listen port |
| `HOST` | `0.0.0.0` | `0.0.0.0` | HTTP listen address |

## HTTP API

Identical surface to the existing Python sidecars:

| Endpoint | Auth | Methods | Response |
|---|---|---|---|
| `/status` | none | GET | `{ running, ready, players, hostname, map, timestamp }` — `hostname` and `map` are empty string when not available |
| `/restart` | Bearer | GET, POST | `{ pid, status }` |
| `/stop` | Bearer | POST | `{ status: "stopped" }` |

## Process Lifecycle

- `main.go` calls `downloadData()` at startup, then `startGame()`
- `startGame()` spawns the game process with `GAME_CMD` + `GAME_ARGS`, keeping stdin open
- `exitGame()` writes `GAME_QUIT_CMD + "\n"` to stdin, waits `GAME_QUIT_TIMEOUT` seconds, then calls `Process.Kill()` if still running
- Signal handlers (`SIGINT`, `SIGTERM`) call `exitGame()` then `os.Exit(0)`
- Auto-shutdown loop runs in a goroutine: polls player count every 60s, triggers ECS `UpdateService(desiredCount=0)` after `IDLE_TIMEOUT_SECONDS` of zero players

## Auto-shutdown (AWS SDK)

Uses the AWS SDK for Go v2 (`github.com/aws/aws-sdk-go-v2`). Credentials come from the standard SDK credential chain (instance role / task role — same as boto3 did before).

## CI: New Sidecar Workflow

`.github/workflows/publish-sidecar.yml`:
- Triggers on `push` to `main` with path filter `sidecar/**`
- Builds with `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o sidecar ./...`
- Creates/updates a GitHub Release tagged `sidecar-latest`, uploading the `sidecar` binary as a release asset

## Dockerfile Changes

Both `docker-containers/xonotic/Dockerfile` and `docker-containers/qssm/Dockerfile`:

**Remove:**
- `COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv`
- `COPY sidecar-service/pyproject.toml .`
- `COPY sidecar-service/sidecar-service.py .`
- `RUN uv sync --no-install-project --python 3.12.3`
- `ENTRYPOINT ["/opt/.venv/bin/python"]` / `CMD ["-u", "sidecar-service.py"]`

**Add:**
```dockerfile
ARG SIDECAR_VERSION=latest
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates \
    && curl -fsSL \
       "https://github.com/fogo-sh/insta-game/releases/download/sidecar-latest/sidecar" \
       -o /usr/local/bin/sidecar \
    && chmod +x /usr/local/bin/sidecar \
    && apt-get purge -y curl \
    && rm -rf /var/lib/apt/lists/*
ENTRYPOINT ["/usr/local/bin/sidecar"]
CMD []
```

Note: `ca-certificates` is already present in both images for game downloads; `curl` is added and then purged to keep the layer lean.

## Pulumi Changes (`pulumi/game_service.py`)

Add the new game-specific env vars to the container definition's `environment` list. `GameService.__init__` gains new parameters:

```python
protocol: str,           # "xonotic" or "quake1"
game_cmd: str,           # e.g. "./xonotic-linux-arm64-dedicated"
game_args: str = "",     # space-separated args
game_quit_cmd: str = "quit",
game_quit_timeout: int = 15,
```

These are passed through as env vars alongside the existing ones. `__main__.py` is updated to pass the correct values for each `GameService` instantiation.

## Makefile Changes

Both `docker-containers/xonotic/Makefile` and `docker-containers/qssm/Makefile`:
- Remove `sidecar-service` from `RUFF_TARGETS` (no Python there anymore)

A new `sidecar/Makefile` (or just documented commands) for local Go development:
```sh
make build   # GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o sidecar .
make test    # go test ./...
```

## Deletions

- `docker-containers/xonotic/sidecar-service/` (entire directory)
- `docker-containers/qssm/sidecar-service/` (entire directory)

## Out of Scope

- No changes to Lambda handler
- No changes to VPC/IAM/ECS cluster resources
- No changes to game server configs or data download format
- No change to the HTTP API shape visible to the Lambda
