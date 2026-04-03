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

The container does not bundle UT99 GOTY assets or binaries. Provide them at
runtime through `DATA_URL` as a zip file that extracts into `/opt`, with this
layout:

```text
System64/
  ucc-bin-amd64
Maps/
  DM-Deck16][.unr
Textures/
Music/
Sounds/
data/
  UnrealTournament.ini    # optional, overrides the bundled default config
```

If the zip does not include `data/UnrealTournament.ini`, the sidecar copies the
bundled default config to `/opt/data/UnrealTournament.ini`.

The container is built for `linux/amd64`, and the startup wrapper expects the
dedicated server executable at `/opt/System64/ucc-bin-amd64`.
