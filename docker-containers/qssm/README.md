# QSS-M Server Container

This folder builds a QSS-M dedicated server container controlled by the same
sidecar pattern as the Xonotic container. The image builds QSS-M from source so
it works on both amd64 and arm64 Docker hosts.

## Build and run

```sh
make build
make run
```

The local server listens on UDP `26000` and the sidecar listens on TCP `5001`.

## Required game assets

The container does not bundle the commercial Quake data files. Provide them at
runtime through `DATA_URL` as a zip file that extracts into `/opt`, with this
layout:

```text
id1/
  pak0.pak
  pak1.pak
  server.cfg    # optional, overrides the bundled default config
```

`pak0.pak` and `pak1.pak` come from a purchased Quake installation. Keep the
directory and filenames lowercase (`id1/pak0.pak`, `id1/pak1.pak`) so Linux
servers and QSS-M clients can load them consistently.

For local-only smoke testing without `DATA_URL`, the sidecar copies the bundled
`server.cfg` to `/opt/id1/server.cfg`, but the server will still fail to start
until valid `pak0.pak` and `pak1.pak` files are present in `/opt/id1`.

## RCON

Set `RCON_PASSWORD` in your local `.env` before `docker compose up qssm`, or
use `QSSM_RCON_PASSWORD` if you want a QSS-M-specific override. In production,
set the Pulumi secret `rconPassword`, with `qssmRconPassword` as an optional
QSS-M-only override.

From a QSS-M client console:

```text
rcon_password your-password
```

## Client install notes

Use the QSS-M release packages for clients, then copy the same lowercase
`id1/pak0.pak` and `id1/pak1.pak` files into the Quake base directory.

The container already bundles QSS-M's engine pak files (`quakespasm.pak` and
`qssm.pak`) from the source build.

Official QSS-M downloads and docs:
https://qssm.quakeone.com/
