# Repository Guidelines

The application is one Go module. `cmd/insta-game` is the entrypoint;
`internal/command` contains CLI dispatch; `internal/control` owns instances,
providers, the API, Discord interactions, and the embedded web UI.
`internal/agent` supervises one game and `internal/protocol` queries engines.
`internal/wake` handles Quake cold-start requests; `internal/securehttp` contains
token-pinned TLS. `internal/cloud` provisions a shared EC2 host with its embedded
CloudFormation template. `docker-containers/*` contains engine image builds,
game metadata, and default configurations.

Keep lifecycle policy in the controller. Providers manage their own resources;
agents do not provision infrastructure. Verify ownership before mutations.
Preserve independent instances, private credentials, stop-retains-data behavior,
and actual failure reporting. Prefer simple code and deletion over compatibility
layers. Do not reintroduce another application runtime or deployment program.

From the repository root:

```sh
make build
make test
make check
python3 generate-compose.py
python3 generate-compose.py --check
```

`compose.yml` is generated from game metadata. Do not edit it manually. Its game
services are image-build targets; the Go controller creates running instances.
Run the generator after modifying `game.json`.

Game image preparation uses `make download`, `make clean`, and `make build`
inside the engine folder. Build images before creating Docker instances.
Native process supervision currently requires Linux.

For AWS, review `insta-game aws plan` before `aws apply`. Use an isolated stack
for integration checks, track created resources, and verify deletion. Do not
rename or destroy pre-existing deployed resources as a side effect of source
migration. Live tests require operator credentials and are separate from the
ordinary test suite.

Run `go test -race ./...`, `go vet ./...`, the Compose freshness check, and
appropriate real provider tests before finishing. Preserve the user's running
game servers unless changing them is part of the current request.
