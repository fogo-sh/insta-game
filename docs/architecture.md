# Architecture

One Go module builds `insta-game`. A game catalog entry describes an engine;
an instance owns its selected provider, immutable catalog/config snapshot,
credentials, desired state, observations, and activity policy. Several instances
can use the same game without sharing runtime identity or writable data.

## Responsibilities

| Package | Responsibility |
| --- | --- |
| `cmd/insta-game` | Entrypoint and signal handling |
| `internal/command` | CLI, controller startup, AWS operator commands, Discord registration |
| `internal/control` | Instance store, reconciliation, providers, authenticated API, Discord interactions, embedded UI |
| `internal/agent` | One game process, startup assets/config, readiness, console, logs, authenticated management |
| `internal/protocol` | Engine status queries and player counts |
| `internal/cloud` | Shared EC2 host provisioning, SSH installation, wake/stop/destroy |
| `internal/wake` | Quake connection rejection and deduplicated cold-start callbacks |
| `internal/securehttp` | Token-pinned TLS for controller and agent clients |

The controller owns lifecycle policy. Providers implement `Ensure`, `Inspect`,
`Start`, `Stop`, and `Delete`. The agent supervises its child and reports game state.
AWS infrastructure operations run on the operator machine; the shared host
needs no AWS credentials.

## State and recovery

Creation commits intent to bbolt before allocating resources. A stable request
ID supports retries with the same settings. Up to four independent instances
reconcile concurrently. Provider calls have deadlines; generation checks prevent
an old observation from replacing newer user intent. Errors remain visible.

Readiness requires a game query, not merely a running process or container.
Unknown/unready activity resets the idle interval. Maximum runtime is a separate
bound. Timeout decisions run only while the controller is online. Unexpected
exits require explicit stop/start.

The database contains private credentials and saved instance settings. Keep it
with its provider configuration and run one controller per database. Deletion
retains a tombstone while removing owned runtime resources and data. Ownership
verification is required before destructive operations.

## Provider lifecycle

**Local:** Linux runs a detached agent per instance. An ownership file and held
filesystem lock identify it without trusting a reused PID. The provider copies
operator-supplied assets into a private working directory and allocates ports.
Stop leaves the agent and files available; start restarts the child; delete
shuts down the agent before removing its directory. A killed Linux agent also
kills its direct child. Native commands and argument placeholders are described
in the [README](../README.md#native-linux-games).

**Docker:** Each instance owns a container and its writable layer. Labels bind
ownership to the instance credentials; mutations target the verified immutable
container ID. Host ports are allocated independently, with matching external
ports for TCP/UDP pairs. Agents are published on host loopback. Stop retains the
container; delete removes it. The controller must share the Docker host's
network view; a remote daemon is unsupported.

## Shared AWS host

One CloudFormation stack provisions a public subnet and one EBS-backed EC2 VM.
Several native games share its CPU and memory while retaining independent
working directories, ports, and lifecycle state. The VM runs the same controller
and local provider as a workstation. There is one authoritative game database,
on its persistent disk.

Normal EC2 stop retains the disk. Start refreshes the auto-assigned public IP;
native endpoint observations use the current host address with saved ports.
Destroy removes the VM and its disk, so export anything needed first.

The controller's optional `--poweroff-idle` flag powers off only after every
saved game is fully stopped or deleted for the complete grace period. Failed,
unknown, running, or pending states prevent idle shutdown. Game idle policy
still counts human players where the protocol supports it. An independent
six-hour systemd boot timer bounds the experiment if the controller crashes.

A separate process on an existing operator host listens for valid NetQuake
connect requests. It deduplicates concurrent wake attempts, starts EC2, waits
for authenticated controller readiness, starts the selected saved game, and
returns its direct address through a Quake rejection message. Queries cannot
wake the VM. Gameplay goes directly to EC2; this is not a transparent UDP relay.
A short ready-address cache expires before later attempts revalidate the host.

## Persistent game roots

Native instances copy operator-supplied assets into private directories on the
host disk. Docker images seed their `/opt` game root from
`/usr/local/share/insta-game/seed`; their writable layer survives container stop.
Seeding preserves existing files. Engine upgrades need an explicit data upgrade
or a fresh instance.

## Transport and API

Local and Docker agents use loopback HTTP with bearer authentication. The
shared host controller uses TLS with a token-pinned public key plus bearer
authentication. SSH and controller TCP port 3000 accept only the operator CIDR;
game UDP ports are public. The installer verifies SSH against the host key
reported through the authenticated EC2 console API.

All `/api/` routes require `Authorization: Bearer <controller-token>`:

| Method | Route | Operation |
| --- | --- | --- |
| GET | `/api/catalog` | Game definitions and default configurations |
| GET | `/api/providers` | Configured providers and default |
| GET / POST | `/api/instances` | List / create instances |
| GET / DELETE | `/api/instances/{id}` | Read / request permanent deletion |
| POST | `/api/instances/{id}/start` | Request start |
| POST | `/api/instances/{id}/stop` | Request stop |
| GET | `/api/instances/{id}/logs` | Stream plain-text SSE events |
| POST | `/api/instances/{id}/console` | Send `{ "command": "status" }` to game stdin |

Creation accepts `id`, `provider`, `game`, `name`, `config`, `idleMinutes`, and
`maxMinutes`. ID, provider, and config can be omitted. Zero timeouts disable
limits; clients choose their defaults. Mutations return HTTP 202 before provider
work completes. Ordinary responses omit instance credentials.

The embedded UI and CLI use these routes. Discord `/discord` verifies signed
interactions and an allowed guild, then calls the same instance operations.
There is no separate launcher or deployment application. Fly and automatic
adoption of earlier deployed resources are outside the implementation.
