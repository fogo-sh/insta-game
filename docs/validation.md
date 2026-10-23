# Validation

`make test` runs Go race tests and UI settings tests. `make check` runs
CloudFormation lint, vet, and Compose freshness. CI validates Compose and packages
Linux AMD64/ARM64 and macOS ARM64 binaries.

Socket tests cover loading, handshake replay, reliable sequence translation,
independent clients, admission limits, cancellation, idle expiry, failure, and
shutdown. A controller/gateway integration test uses the real API, database,
reconciliation loop, and UDP relay with simulated infrastructure and game status.
These checks need neither Docker nor AWS.

## Real clients against AWS

The six games below were tested with real graphical clients on Linux against
servers on an isolated AWS `t4g.small` host with a 32 GiB disk. Each case stopped
EC2 first and added a 45-second engine delay. One client connection woke the host,
waited, loaded the game, and accepted movement/fire input without reconnecting.
Recorded joins took roughly 66 to 83 seconds, including the deliberate delay.

| Game | Client tested | Waiting behavior |
| --- | --- | --- |
| Quake II | Official q2repro Win64 `2f77156`, Wine 11, matching upstream game DLL | Connection progress |
| Quake III | Ubuntu ioquake3 `3ef30e75` | Connection progress |
| OpenArena | OpenArena 0.8.8 with its distribution's ioquake3 settings | Connection progress |
| Xonotic | Xonotic 0.8.6 Linux SDL | Loading messages, then DarkPlaces 7 sign-on |
| UT99 | OldUnreal 469f RC6 Linux | Connecting screen stays alive |
| BZFlag | BZFlag 2.4.30 Linux against 2.4.26, protocol 0221 | Server progress messages |

Quake II used demo data and a game DLL built from q2repro's matching upstream
submodule. Quake III used demo data, official patch data, and upstream game
modules. Client engine binaries were unchanged. These are specific client/server
pairs: Yamagi cannot join the catalog's current q2repro server protocol.

UT99 also passed a 155-second engine delay on the running AWS host, joining after
about 165 seconds with its default 150-second initial timeout unchanged. A failed
start produced the stock client's `Rejected by server` error and closed the
pending connection.

BZFlag also passed two simultaneous graphical clients, a disconnect, and a
replacement join with backend slot zero reserved to force player-ID translation.
The server reported two players, then one, then two again; both clients continued
receiving updates through the gateway.

Existing Quake 1 coverage includes real QSS-M/vkQuake clients through AWS cold
wake, a 45-second loading hold, movement, firing, concurrent players, and map
changes. Native/Docker lifecycle checks cover independent instances, retained
files, controller restart, stop/start, ownership, and deletion. AWS Docker BZFlag
also passed host reboot recovery and retained-data checks.

## Limits

These runs check startup and basic gameplay, not long matches, every mod, or
capacity under load. BZFlag replay and shared-process `-solo` robots are excluded;
its supported wire version is 0221. QuakeWorld is outside the NetQuake loading
path. UT99's extra browser/query ports require direct hosting. Commercial assets
are operator-supplied and excluded from source and release bundles.

Native supervision requires Linux. The supplied images target ARM64; the
controller binary must match its host. Discord signatures are tested locally;
live command registration is untested. Fly is not implemented.

Run the Docker lifecycle check with a catalog containing a built Xonotic image:

```sh
INSTA_GAME_DOCKER_CATALOG=/path/to/catalog go test -v ./internal/control \
  -run TestDockerTwoServersSurviveControllerRestart -count=1 -timeout=4m
```
