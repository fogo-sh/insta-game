# BZFlag ARM64 Server Container

This folder builds an ARM64-only BZFlag server container controlled by the
same sidecar lifecycle API as the other game containers.

## Build and run

```sh
make build
make run
```

The local game server listens on TCP/UDP `5154` and the sidecar listens on TCP
`5001`.

## Server config

A private random-map config is bundled at `/opt/default-server.cfg`. To supply
a custom config at runtime, set `DATA_URL` to a raw config file URL targeting
`/opt/data/server.cfg`, for example:

```sh
DATA_URL="https://example.com/server.cfg=/opt/data/server.cfg"
```
