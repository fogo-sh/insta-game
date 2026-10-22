# Go implementation and validation

One `insta-game` executable contains the controller, CLI, web UI, Discord
integration, game agent, AWS host installer, and Quake wake listener. The
TypeScript launcher, separate sidecar executable, Python deployment app, and
per-game Fargate/EFS implementation have been removed. Fly is not implemented.

## Runtime

- `local`: native Linux engines with explicit executable/asset configuration,
  detached agents, independent directories, and allocated ports.
- `docker`: owned containers with independent writable data and published ports.
- AWS: one shared EC2 host runs the same controller and providers. Encrypted
  gp3 retains the database and game data while EC2 is stopped. The operator's
  Go binary provisions, installs, starts, stops, and destroys the host.
- Optional Quake wake listener on an existing host. Connection requests trigger
  a bounded wake; queries do not. Rejection messages report startup or a direct
  game address. It does not proxy gameplay.

The shared controller owns game idle limits. Its host idle option powers off
only after all games are stopped/deleted for a complete grace period. A separate
six-hour boot timer limits failed experiments. No AWS credentials are installed
on the game host. SSH uses a host key obtained through the authenticated EC2
console API; controller HTTPS pins a key derived from its private token.

## Validation

Earlier native and Docker validation covered two real FTEQW/Xonotic instances,
distinct endpoints and data, controller restart, stop/start persistence, and
independent deletion. The user tested vkQuake against FTEQW.

The old isolated AWS experiment was fully destroyed on 2026-10-22. Its EFS,
ECR repository, asset bucket, execution role, VPC, and active CloudFormation
stacks were independently checked as absent.

Shared-host checks use three real FTEQW servers with distinct dm1/dm2/dm3
configs, two FrikBots each, and public UDP queries. The 1 GiB t3a.micro retained
about 500 MiB available with these bot-only servers. This is not a full human
player load test. Native FTE startup uses an explicit 64 MiB QuakeC arena;
its 2 GiB default cannot start reliably on this small host.

Live checks also covered independent stop/start while the other two remained
queryable, a game PID surviving controller restart, game files retained through
EC2 stop/start, and fresh public connection addresses after restart. Review
fixes cover admission races during idle shutdown, boot identity recovery, and
systemd startup ordering. The empty-game timeout stopped a real bot-only game;
the host then powered off after its two-minute grace period and EC2 reached
`stopped`, releasing its public IPv4. A later valid tailnet connection attempt
returned the ready direct address in 22 seconds without manual intervention;
status queries left the stopped host asleep. Only the requested saved game
started; the other two remained stopped. Its saved file survived both cold
starts. The temporary idle-test instance was deleted independently.
Automated checks are `make test` and `make check`; provider networking and
playability require real runtime checks beyond unit tests. Discord checks use
signed local requests; live registration has not been performed.
