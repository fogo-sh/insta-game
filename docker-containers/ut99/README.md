# UT99 GOTY server image

This Linux ARM64 image runs a supplied UT99 dedicated server through
`insta-game agent`. From the repository root:

```sh
make build
docker compose --profile images build ut99
export UT99_DATA_URL=https://YOUR_ASSET_HOST/ut99-assets.zip
bin/insta-game serve
# In another terminal:
bin/insta-game create --provider docker --game ut99 --name "UT99 night"
```

Supply your own compatible UT99 assets and ARM64 server executable. The ZIP
extracts into `/opt` with this layout:

```text
SystemARM64/
  ucc-bin-arm64
Maps/
  DM-Deck16][.unr
Textures/
Music/
Sounds/
```

The startup wrapper expects `/opt/SystemARM64/ucc-bin-arm64`. The controller
supplies the default `UnrealTournament.ini` at `/opt/data/UnrealTournament.ini`;
use `create --config /path/to/UnrealTournament.ini` for a custom configuration.

The catalog publishes UDP 7777 to 7781, including game port 7777 and query port
7778. Docker allocates external ports. Check the returned endpoints and validate
client connectivity for the selected provider. Agent TCP 5001 is managed by
the provider.

The full `/opt` root, including downloaded binaries/assets, is retained across
stop/start in the container's writable layer. See the
[running servers](../../docs/running.md) for hosting and deletion.
