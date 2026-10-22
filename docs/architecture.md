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
| `internal/cloud` | AWS environment configuration and embedded CloudFormation bootstrap |

The controller owns lifecycle policy. Providers implement `Ensure`, `Inspect`,
`Start`, `Stop`, and `Delete`; asynchronous work returns a pending observation
or `ErrPending`. The agent supervises its child and reports game state. Its AWS
credentials are limited to instance storage and asset reads; it does not create
infrastructure or decide when to stop an instance.

## State and recovery

Creation commits intent to bbolt before allocating resources. A stable request
ID supports retries with the same settings. Up to four independent instances
reconcile concurrently. Provider calls have deadlines; generation checks prevent
an old observation from replacing newer user intent. Errors remain visible.

Readiness requires a game query, not merely a running process or container.
Unknown/unready activity resets the idle interval. Maximum runtime is a separate
bound. Timeout decisions run only while the controller is online. Unexpected
exits require explicit stop/start.

The database contains private credentials and provider attempt records. Keep it
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

**AWS:** A shared CloudFormation environment provides networking, ECS, EFS,
registry, assets, and logs. Each instance has its own CloudFormation stack with
an EFS access point, task role, task definition, and game security group. ECS
runs standalone Fargate tasks. Stop terminates the task but retains the instance
stack and data. Start creates a new task attempt and may change its public IP.

Before submitting a task, the provider persists its attempt identity. Retries
reuse an ECS client token, task ARN, and `startedBy` identity; terminal results
are saved before ECS task history expires. An unresolved request is reconciled
even if user intent changes to stop or delete. Missing history requires a search
for live attempts before the attempt can be closed.

AWS deletion stops the game, runs an owned cleanup task against only that
instance's access point, verifies a successful exit, then deletes the instance
stack. Removing an access point alone would leave its files. Failed cleanup
keeps the instance visible and can be retried. Environment destruction is a
separate operator command and refuses remaining game stacks/tasks.

## Persistent game roots

Images place a complete seed of their `/opt` game root at
`/usr/local/share/insta-game/seed`, outside the runtime data mount. The agent claims
the root, copies missing seed files, records completed seeding, downloads any
configured assets, applies launch configuration, then starts the engine.

On AWS, the instance's EFS access point mounts at `dataRoot` (default `/opt`).
Persisting the full root keeps downloaded mods and other engine files alongside
configuration, rather than losing everything outside a narrow config directory
on task replacement. The owner marker scopes cleanup; seeding does not overwrite
existing files. Image changes therefore need a deliberate game-data upgrade or
a new instance. Docker retains this root in its writable layer. Native instances
copy their operator-supplied asset directory instead.

## Transport and API

Local and Docker agents use loopback HTTP with bearer authentication. AWS agents
use TLS with a public key pinned to their private instance token, plus bearer
authentication. Cloud management TCP port 5001 is restricted to the controller's
configured public IPv4 CIDR; game ports are public. Bootstrap provisions game
hosting resources, not the controller host or an HTTPS frontend for its UI.

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
work completes. Ordinary responses omit instance credentials and private AWS
attempt state.

The embedded UI and CLI use these routes. Discord `/discord` verifies signed
interactions and an allowed guild, then calls the same instance operations.
There is no separate launcher or deployment application. Fly and automatic
adoption of earlier deployed resources are outside the implementation.
