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
| `internal/wake` | Game wake routing, NetQuake loading, TCP/UDP relay, and deduplicated starts |
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
Native games and Docker containers share its CPU and memory while retaining independent
working directories, ports, and lifecycle state. The VM runs the same controller
and providers as a workstation. There is one authoritative game database,
on its persistent disk.

Normal EC2 stop retains the disk. Start refreshes the auto-assigned public IP;
native endpoint observations use the current host address with saved ports.
Destroy removes the VM and its disk, so export anything needed first.

The controller's optional `--poweroff-idle` flag powers off only after every
saved game is fully stopped or deleted for the complete grace period. Failed,
unknown, running, or pending states prevent idle shutdown. Game idle policy
still counts human players where the protocol supports it. An independent
systemd boot timer bounds host lifetime independently of the controller. The
installer defaults it to six hours; `--host-max-runtime` changes or disables it.

A gateway on an existing operator host owns stable player-facing addresses.
It starts saved games through any reachable controller, optionally waking EC2
first. All catalog protocols have a gateway route type. Quake 2, Quake 3,
Xonotic, and UT99 use per-client connected UDP sockets and retain the initial
handshake until startup finishes. Payloads and sequence numbers are unchanged.
Client connection timeouts remain client policy; users retry the same address
if a cold start exceeds them. UT99 GameSpy queries are ignored and additional
query services use direct hosting.

BZFlag accepts its TCP greeting before waking the game, then relays both TCP
and UDP over the same front port. UDP link setup is admitted only while a TCP
connection from that client address exists. Queries share the TCP greeting and
can therefore wake the host. Connections, startup, and idle work are bounded.

For NetQuake, the gateway provides a loading connection before sign-on. It accepts a NetQuake connection immediately and sends reliable loading
messages before game sign-on. The shared wake callback starts EC2, waits for
fresh authenticated controller readiness, and starts the selected saved game.
NetQuake status queries are answered locally without waking anything.

Each player gets a separate backend UDP socket and negotiation. The gateway
waits for the last loading message to be acknowledged, connects to the backend,
and forwards the client's initial unacknowledged reliable packet. Backend
reliable DATA sequence numbers gain the acknowledged loading-message count;
client ACK/NAK numbers lose it. Client reliable DATA and backend ACKs are
unchanged. Unreliable packets and payloads are never renumbered: extended game
protocols refer to their sequence numbers in snapshot acknowledgments.

The client therefore receives the backend's first sign-on over its existing
connection. No waiting map, client modification, automatic reconnect command,
or patched game engine is needed. FTE uses `sv_listen_nq 1`; basic handshakes on
both legs keep ProQuake angle negotiation consistent. Backend rejections and
unsupported transport fail visibly instead of looping.

The listener bounds active clients, packet queues, boot attempts, and idle
sessions. Wake work is shared by game ID; one disconnect cannot cancel another
client's startup. Canceling the gateway closes its listeners, backend sockets,
and workers. Backend game ports use `--game-cidr`, set to the gateway public
IP when proxying. SSH and the controller API use `--operator-cidr`.

This uses the intermediary/session pattern from notscared's Elixir master_server.
Its custom Quake II transfer packets and engine hooks are not part of this
implementation; holding the client before its initial sign-on avoids needing
that engine-specific transfer mechanism.

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
game UDP uses the separate player/gateway `--game-cidr` restriction. The
installer verifies SSH against the host key reported through the authenticated
EC2 console API.

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
