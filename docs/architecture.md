# Architecture

This describes the current implementation. The [Go unification design](go-unification.md)
sets the completion criteria: local, Docker, and AWS in one executable, with
the superseded runtime and deployment code removed before merge.

The game catalog describes what can be launched. An instance describes one
particular server: immutable game/config snapshot, provider, allocated endpoints,
per-instance credentials, desired state, observed phase, and activity policy.
Game IDs never identify running infrastructure.

## Packages

- `cmd/insta-game`: signal handling and executable entrypoint.
- `internal/command`: CLI and controller startup.
- `internal/control`: catalog, transactional instance store, provider boundary,
  reconciliation, API, and embedded web assets. Its Docker provider owns
  container allocation and connection discovery.
- `internal/agent`: one child process, readiness queries, configuration, logs,
  and authenticated management endpoints. It owns no cloud provisioning logic.
- `sidecar/protocol`: engine query implementations shared by the agent.
- `sidecar/main.go`: compatibility entrypoint for the existing AWS images.
- `launcher/`, `pulumi/`: existing deployment path, preserved independently.

The Go controller owns cloud/hosting credentials. A container gets only its
agent token and game administration password. Mutations verify Docker ownership
and target the inspected immutable container ID. Per-instance writable layers
replace the legacy shared per-game bind mounts.

## Reconciliation

Creation persists intent before any provider operation. An optional stable
request ID makes retries idempotent. One controller process owns the bbolt file;
separate instances reconcile concurrently with a limit of four. Each provider
operation has a deadline. A generation counter prevents stale observations from
replacing a newer user request.

Stopping retains data. Deletion leaves a database tombstone for request history
and idempotency, but removes the container and writable data. A stopped or failed
server does not become an absent game; it remains an independently managed
instance. A failed observation is recorded as an error, never inferred to mean
that infrastructure is stopped or there are no players.

Readiness is different from container liveness. A successfully started container
that subsequently exits is failed, including failures during startup. Retrying
requires stop/start. Idle timeout requires consecutive confirmed empty samples;
unknown or unready samples reset the empty interval. Maximum runtime provides a
separate bound when readiness cannot be established. Timeout enforcement resumes
when a controller restarts; it does not run while the controller is down.

## Scope and next provider

The implemented runtime providers are local Docker and Linux native processes.
The retained AWS stack
still uses singleton ECS services; it is not a multi-instance provider for the
Go controller. There is no Fly adapter or automatic migration of existing
servers. No cloud resources are provisioned by this change.

A second provider should implement the small infrastructure boundary in
`model.go` and return actual game endpoints plus an agent address reachable by
the controller. It should not add provider branches to HTTP handlers or the CLI.
The runtime API creates/stops game instances; account bootstrap (networks,
permissions, registries) remains a separate operator concern. Avoid writing an
infrastructure framework before a second provider proves what is shared.

For Fly Machines, validate two independently reachable same-game servers before
committing to resource layout. Current images are ARM64, while Fly currently
runs x86 hardware. Public UDP requires a dedicated IPv4, binding the engine to
`fly-global-services`, and matching internal/external ports. Verify auxiliary
ports, per-instance routing, stop/start, writable storage, architecture, and
address costs. Relevant sources:

- https://docs.fly.io/networking/udp-and-tcp
- https://community.fly.io/t/x86-architecture-guarantee/28811

Portable Go binaries do not make native game engines portable. Multi-architecture
game image builds, cloud storage/backups, provider bootstrap, account-based
ownership, native RCON clients, and Discord on the new instance API remain
separate follow-ups. The existing TypeScript Discord integration remains usable
on the legacy deployment.

## Native processes

The `local` provider runs a detached `insta-game agent` per instance on Linux.
The agent holds a filesystem lock while alive, so a controller restart can
rediscover it without trusting a reused PID. Stopping keeps the agent available
and stops its game; deleting shuts down the agent and removes its owned files.
A killed Linux agent also kills its direct game child. macOS builds currently
support the Docker controller, not native process supervision.

Enable native execution with `serve --native-config /path/to/native.json`.
Each configured engine supplies an absolute executable, an absolute directory
of assets to copy, a relative config path inside that copy, and explicit args:

```json
{
  "fteqw": {
    "command": "/home/jack/games/fteqw.sv",
    "assets": "/home/jack/games/quake-assets",
    "configPath": "id1/server.cfg",
    "args": ["-dedicated", "8", "-nohome", "-basedir", "{data}",
             "-game", "id1", "-port", "{port}", "+exec", "server.cfg"]
  }
}
```

Assets must contain regular files and directories; symlinks are rejected.
Copying costs disk space and startup time, but prevents games from modifying a
shared asset directory. `{data}` is the private working directory; `{config}`
is its absolute config filename. `{port}` is the allocated primary game port;
`{port:27960}` names another port from the game metadata. Configure every engine
port explicitly when its defaults would collide with another instance.

Use `create --provider local ...` or choose `local` in the existing create form.
`--default-provider local` changes the default; saved instances retain their
provider. The controller exposes only configured providers through
`GET /api/providers`. The agent configuration and credentials stay in private
files under the controller's data directory.

## Agent transport

Native and Docker agents use host loopback. Cloud agents can enable
`AGENT_TLS=true`. The controller pins the TLS public key to the instance's
private management token, so a changing task IP does not require public DNS or
a public certificate authority. A different instance's certificate is rejected
before sending credentials. Plain bearer authentication is still required after
the encrypted connection is established. Restrict cloud management ports to
the controller's network address as an additional boundary.
