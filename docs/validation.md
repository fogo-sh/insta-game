# Validation scope

The controller, CLI, web UI, agent, AWS installer, and wake gateway build as one
Go application. Native supervision requires Linux. The supplied game images
are built for ARM64; an AWS host and controller must match that architecture.

## Automated checks

`make test` runs Go race tests and the UI settings tests. `make check` runs pinned
CloudFormation lint, vet, and Compose freshness. CI also validates Compose and
cross-compiles Linux ARM64 and macOS ARM64.

Gateway network tests cover every catalog protocol. They exercise real UDP and
TCP sockets, startup buffering, independent players, source filtering, capacity,
cancellation, failure cooldown, and shutdown. NetQuake tests additionally cover
lost loading ACKs, fragmented reliable messages, sequence translation, and
unchanged unreliable traffic. BZFlag coverage exercises its TCP greeting and UDP
link on one stable front port. These protocol tests use fake backends; real engine and gameplay coverage is
listed below.

Provider regressions cover resource ownership, retained instance data, native
port offsets, and recovery across host boots while preserving same-boot crash
failures. CloudFormation checks enforce shared-host cost and access boundaries.

## Runtime checks

- Native FTEQW and Docker Xonotic: independent instances, endpoints, and data;
  controller restart; stop/start persistence; owned deletion.
- AWS native FTEQW: shared-host operation, retained files through EC2 stop/start,
  fresh addresses after reboot, game idle timeout followed by host poweroff,
  and controller restart without killing active games.
- Real QSS-M and vkQuake clients: cold-start NetQuake loading and automatic
  sign-on through one connection, movement, firing, concurrent players, and
  backend map changes. A 45-second loading hold exceeded normal client timeout.
- Real local OpenArena: a stopped game started automatically from a gateway
  challenge and replied through the stable address; status traffic followed over
  the same client socket.
- Live ARM64 AWS Docker BZFlag: verified installation on a `t4g` host, game
  readiness, host stop/start recovery, stable-address cold wake with TCP query
  and UDP link negotiation, retained writable data, and a configurable boot
  lifetime timer. The isolated stack was destroyed after validation.
- Real local BZFlag: a stopped game started from the TCP greeting, returned its
  server/player query through the gateway, and completed a UDP link exchange on
  the same front port. These engine checks do not establish full human gameplay.

## Remaining release validation

Quake 2, Quake 3, UT99, BZFlag, and OpenArena still need human gameplay acceptance
on the intended engine/client versions. Xonotic needs human gateway acceptance;
its native/Docker lifecycle has been exercised. The ARM64 AWS Docker deployment was checked with BZFlag; the remaining catalog
images need runtime acceptance on their intended hosts. Commercial game assets
are operator-provided and are not included in the repository or release bundle.

The gateway preserves the address for every catalog protocol. Seamless loading
messages are specific to NetQuake. Other clients can exhaust their own connection
retries while a host boots and must retry the same address. BZFlag TCP queries
can wake a host because queries and joins share their initial handshake. UT99's
additional browser/query services are not proxied. QuakeWorld is not supported
by the NetQuake loading path.

Discord uses signed local interaction validation; live registration has not
been performed. Fly is not implemented. Resource measurements from small bot-only
Quake servers do not establish capacity for other games or full human matches.
