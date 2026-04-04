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

The container bundles Omicron Bot, but not the commercial Quake data files.
Provide the Quake assets at runtime through `DATA_URL` as a zip file that
extracts into `/opt`, with this layout:

```text
id1/
  pak0.pak
  pak1.pak
  server.cfg    # optional, overrides the bundled default config
```

`pak0.pak` and `pak1.pak` come from a purchased Quake installation. Keep the
directory and filenames lowercase (`id1/pak0.pak`, `id1/pak1.pak`) so Linux
servers and QSS-M clients can load them consistently.

The image downloads `obots102-fixed.zip` at build time and installs it to
`/opt/omicron`, so QSS-M can launch with `-game omicron` out of the box.

For local-only smoke testing without `DATA_URL`, the sidecar copies the bundled
`server.cfg` to `/opt/id1/server.cfg`, but the server will still fail to start
until valid `pak0.pak` and `pak1.pak` files are present in `/opt/id1`.

## RCON and bot control

Set `RCON_PASSWORD` in your local `.env` before `docker compose up qssm`, or
use `QSSM_RCON_PASSWORD` if you want a QSS-M-specific override. In production,
set the Pulumi secret `rconPassword`, with `qssmRconPassword` as an optional
QSS-M-only override.

From a QSS-M client console:

```text
rcon_password your-password
rcon addbot
rcon removebot
rcon removeallbots
```

Keep bot count at 0 when you want the sidecar idle shutdown to scale the
service back to zero; Omicron bots may be counted as active players by the
server query.

## Client install notes

Use the QSS-M release packages for clients, then copy the same lowercase
`id1/pak0.pak` and `id1/pak1.pak` files into the Quake base directory.

The container already bundles QSS-M's engine pak files (`quakespasm.pak` and
`qssm.pak`) from the source build.

Official QSS-M downloads and docs:
https://qssm.quakeone.com/
