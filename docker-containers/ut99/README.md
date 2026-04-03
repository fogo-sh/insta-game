# UT99 GOTY Server Container

This image runs a UT99 GOTY dedicated server under the shared sidecar lifecycle
API.

## Build and run

```sh
make build
make run
```

The local game server listens on UDP `7777`, UT's query port listens on UDP
`7778`, and the sidecar listens on TCP `5001`.

## Game assets

The Docker build downloads and installs UT99 GOTY with OldUnreal's Linux
installer, then copies the installed game into the runtime image.

At runtime, `DATA_URL` is optional and can be used to override
`/opt/data/UnrealTournament.ini` with a custom server config.

The container is built for `linux/amd64`, and the startup wrapper expects the
dedicated server executable at `/opt/System64/ucc-bin-amd64`.
