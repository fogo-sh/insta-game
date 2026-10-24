# Architecture

One Go module builds `insta-game`. Catalog entries describe engines. Each
instance saves its provider, catalog/config snapshot, credentials, desired state,
observations, and activity policy. Instances never share writable data or identity.

| Package | Responsibility |
| --- | --- |
| `cmd/insta-game` | Entrypoint and signals |
| `internal/command` | CLI, controller startup, AWS commands, Discord registration |
| `internal/control` | Store, reconciliation, providers, API, Discord, embedded UI |
| `internal/agent` | One game process, assets/config, readiness, console, logs, authenticated management |
| `internal/protocol` | Status queries and player counts |
| `internal/lobby` | Lambda launch page, durable starts, published status |
| `internal/cloud` | EC2 provisioning, verified SSH installation, wake/stop/destroy |
| `internal/wake` | Wake routing, NetQuake loading, TCP/UDP relay, shared starts |
| `internal/securehttp` | Token-pinned TLS |

## State and providers

The controller commits intent to bbolt before allocating resources. Retrying the
same ID and settings is safe. Four instances can reconcile concurrently. Calls
have deadlines; generation checks reject observations of superseded intent.
Errors stay visible. Keep the private database with its provider configuration
and run one controller per database. Deletion removes owned resources and data
but retains a tombstone.

Providers implement `Ensure`, `Inspect`, `Start`, `Stop`, and `Delete`; the agent
supervises the game. Readiness requires a game query. Unknown activity resets the
idle interval; maximum runtime is a separate limit. The controller must stay
online to enforce policy. Unexpected exits require explicit stop/start.

Linux runs a detached agent per native instance. Ownership files and filesystem
locks prevent trusting reused PIDs. Assets are copied into private directories;
ports are allocated per instance. Stop keeps the agent and files, start restarts
the game, and delete stops the agent before removing files. Killing an agent also
kills its direct child. See [native setup](running.md#native-linux-games).

Docker ownership labels bind each container to instance credentials. Mutations
verify ownership and target immutable container IDs. TCP/UDP pairs share an
external port; agents bind to host loopback. Stop keeps the writable layer;
delete removes it. The controller must share the Docker host's network view;
remote daemons are unsupported.

Images seed `/opt` from `/usr/local/share/insta-game/seed`, preserving existing
files. Native copies and container layers survive stop/start. Engine upgrades
need a data upgrade or a fresh instance.

## AWS

One CloudFormation stack creates a public subnet and an EBS-backed EC2 host.
Games share CPU and memory but keep separate directories, ports, and state.
The disk holds the database. Stop retains it; destroy deletes it. Start can
change the public IP; observations refresh addresses while retaining game ports.

The controller's `--poweroff-idle` grace period starts only when every saved game
is stopped or deleted. Failed, unknown, pending, and running states prevent
shutdown. Human player counts drive game idle policy where supported. A systemd
boot timer caps host lifetime even if the controller stops: six hours by default,
changed or disabled by `--host-max-runtime`.

AWS commands run on the operator's machine. Without the optional lobby, the VM
receives no AWS role.
SSH and API port 3000 use `--operator-cidr`; game TCP/UDP use `--game-cidr`,
usually the gateway's public IP. Installation verifies SSH against the host key
from the authenticated EC2 console API. The controller uses bearer authentication
and TLS pinned to the token. Local agents use authenticated loopback HTTP.

## AWS launch page

`internal/lobby` adds a restricted HTTP page and a controller worker. The same
executable runs as a Go Lambda custom-runtime bootstrap. A Function URL serves
the embedded page and API; one DynamoDB table stores pending starts and sanitized
status for up to 16 approved instance IDs. The controller database remains the
source of truth for games, private credentials, and lifecycle policy.

Lambda saves a request before starting its owned EC2 instance. The host consumes
requests using a table-scoped instance role and publishes observations. Acceptance
is stored atomically with local running intent, so retrying after a lost remote
acknowledgement cannot undo a later stop. Completion requires fresh readiness.
An EventBridge rule retries pending wakes each minute, including requests that
arrive during shutdown. Requests expire after ten minutes.

Friends use a separate invitation key; Lambda stores only its SHA-256 digest.
The public API exposes listing and starting, without administrative operations.
Old observations and sleeping hosts never advertise ready addresses. Gameplay
connects directly to the host, whose public IP can change after waking.

The CloudFormation stack owns the request table, private code bucket, Lambda,
roles, retry rule, and short-retention logs. Deployment stages a bootstrap ZIP
before a reviewed code update. The host AMI is pinned at creation; code updates
reuse the deployed template and reject changes to game infrastructure. Destroy empties only the owned artifact bucket.

## Wake gateway

An existing host provides stable player addresses. The gateway starts saved
games through a reachable controller and optionally wakes EC2 first. Starts are
shared by game ID; one client leaving cannot cancel another's startup. Readiness
requires a fresh authenticated controller observation. Listeners, queues, clients,
startup attempts, and idle sessions are bounded. Cancellation closes sockets and
joins workers.

Quake 2/3 and OpenArena receive out-of-band progress while awaiting the real
challenge. UT99 receives packet -1 keepalives, which refresh receive time without
advancing sign-on sequences. Its initial HELLO is retained. After readiness, each
client relays through its own UDP socket with unchanged payloads and sequences. UT99 GameSpy
queries are ignored; extra query services require direct hosting.

BZFlag sends its protocol greeting and a virtual player ID immediately, then
buffers the client handshake while starting the server. A per-client permutation
translates player IDs in both directions, including shots and flag ownership.
TCP and UDP share a front port; UDP links bind to the TCP client by address and
virtual ID. Queries share the greeting and can wake the host. Support is pinned
to protocol 0221; replay and shared-process robot connections are excluded.

NetQuake clients connect immediately and receive reliable loading messages;
status queries are answered locally without waking anything. After the last
loading ACK, each client negotiates its own backend socket and sends its initial
unacknowledged reliable packet. Backend DATA sequences gain the acknowledged
loading-message count; client ACK/NAK sequences lose it. Client DATA, backend
ACKs, and unreliable packets stay unchanged. Extended game protocols use those
unreliable sequence numbers in snapshot acknowledgments.

Xonotic uses the same hold, but negotiates DarkPlaces 7 with the backend before
relaying. Sign-on continues on the same connection without a waiting map, client changes,
reconnect commands, or engine patches. FTE needs `sv_listen_nq 1`; basic handshakes
on both legs preserve ProQuake angle negotiation. Backend rejection and
unsupported transport fail visibly. The session pattern comes from notscared's
Elixir master_server; holding before sign-on avoids its custom Quake II transfer
packets and engine hooks.

## API

All `/api/` routes require `Authorization: Bearer <controller-token>`:

| Method | Route | Operation |
| --- | --- | --- |
| GET | `/api/catalog` | Game definitions and default configurations |
| GET | `/api/providers` | Configured providers and default |
| GET / POST | `/api/instances` | List / create |
| GET / DELETE | `/api/instances/{id}` | Read / request permanent deletion |
| POST | `/api/instances/{id}/start` | Request start |
| POST | `/api/instances/{id}/stop` | Request stop |
| GET | `/api/instances/{id}/logs` | Plain-text SSE stream |
| POST | `/api/instances/{id}/console` | Send `{ "command": "status" }` to game stdin |

Creation accepts `id`, `provider`, `game`, `name`, `config`, `idleMinutes`, and
`maxMinutes`. ID, provider, and config are optional. Zero timeouts disable limits;
clients choose defaults. Mutations return HTTP 202 before provider work finishes.
Responses omit instance credentials.

The UI and CLI use these routes. `/discord` verifies signed interactions and the
allowed guild, then calls the same instance operations. The same binary runs
launcher and deployment commands. Fly and automatic resource adoption are
unsupported.
