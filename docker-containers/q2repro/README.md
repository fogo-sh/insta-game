# Q2REPRO ARM64 Server Container

This folder builds an ARM64-only Q2REPRO dedicated server container controlled
by the same sidecar lifecycle API as the other game containers.

## Build and run

```sh
make build
make run
```

The local server listens on UDP `26000` and the sidecar listens on TCP `5001`.

## Required game assets

The image does not bundle commercial Quake II data files. Provide them at
runtime through `DATA_URL` as a zip file that extracts into `/opt`, with this
layout:

```text
baseq2/
  pak0.pak
  server.cfg    # optional, overrides the bundled default config
```

Use lowercase directory and file names so Linux servers and clients load them
consistently.

## Client notes

Q2REPRO targets Quake II Enhanced/rerelease compatibility while keeping a
Q2PRO-family client/server stack. Use current Q2REPRO or Q2PRO clients on PC,
and validate your selected game DLL/mod mix before opening the server publicly.
