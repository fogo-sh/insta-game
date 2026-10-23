# BZFlag server image

This Linux ARM64 image runs BZFlag through `insta-game agent`. From the repository root:

```sh
make build
docker compose --profile images build bzflag
bin/insta-game serve
# In another terminal:
bin/insta-game create --provider docker --game bzflag --name "BZFlag night"
```

The engine uses TCP/UDP 5154 internally. Docker publishes both on the same
allocated host port; use the address returned by the controller. Agent TCP 5001
is managed by the provider.

The bundled `server.cfg` runs a private random-map game. Use `create --config
/path/to/server.cfg` for a custom launch configuration. The full `/opt` root is
seeded per instance and retained across stop/start; deletion removes instance
data. See [running servers](../../docs/running.md) for native setup, AWS, storage, and deletion.
