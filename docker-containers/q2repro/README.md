# Q2REPRO server image

This Linux ARM64 image runs Q2REPRO through `insta-game agent`. From the
repository root:

```sh
make build
docker compose --profile images build q2repro
export Q2REPRO_DATA_URL=https://YOUR_ASSET_HOST/quake2-assets.zip
bin/insta-game serve
# In another terminal:
bin/insta-game create --provider docker --game q2repro --name "Quake II night"
```

Supply your own Quake II game data. The ZIP extracts into `/opt` and should
contain lowercase paths such as:

```text
baseq2/
  pak0.pak
```

The image supplies its engine/game library and a default `baseq2/server.cfg`.
Use `create --config /path/to/server.cfg` to select the launch configuration;
the controller applies it after asset preparation. The engine uses UDP 27910
internally. Docker allocates a host port; use the controller's current address.
Agent TCP 5001 is managed by the provider.

The full `/opt` root is seeded per instance and retained across stop/start.
See the [root README](../../README.md) for shared-host setup and deletion.
Test your chosen client and game DLL combination before inviting players.
