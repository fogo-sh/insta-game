# Go migration status

The worktree now uses one `insta-game` executable for the controller, CLI, web
UI, Discord integration, game agent, and AWS environment commands. The former
TypeScript launcher, separate sidecar executable, Python deployment application,
and their deployment workflows have been removed. Fly is not implemented.

## Implemented scope

- `local`: native Linux engines with explicit executable/asset configuration,
  detached agents, independent directories, and allocated ports.
- `docker`: independent owned containers, isolated writable data, and discovered
  connection addresses.
- `aws`: CloudFormation environment plan/apply/config/destroy, private asset
  upload, image push, and independent Fargate task attempts with EFS storage.
- Shared desired-state lifecycle, activity policy, private credentials,
  configuration snapshots, console/log access, and simple embedded UI.
- Go Discord create/start/stop/status commands operating on instance IDs.

The controller runs on an operator-managed host. AWS bootstrap creates game
hosting resources; it does not deploy the controller. Public cloud management
uses pinned TLS, bearer authentication, and a restricted source CIDR. Each AWS
instance mounts its full game root through a dedicated EFS access point. Images
seed that root from a copy kept outside the mount. Deletion verifies scoped
cleanup before removing the instance stack.

Removing source code does not retire existing deployments. Inventory and remove
older live resources separately; the new providers do not adopt them by name.

## Validation status

Validated on 2026-10-22:

- Native Linux: two real FTEQW servers, distinct ports/configs, controller
  restart, retained files across stop/start, and independent deletion.
- Docker: two real Xonotic servers, separate configs/endpoints, controller
  recovery, independent stop/start, and deletion.
- Seed-only FTEQW image runtime: populated an initially empty game root,
  answered Quake UDP queries, and retained data through stop/start.
- AWS ca-central-1: two real FTEQW servers answering public Quake UDP queries;
  independent stop/start; a saved file outside the config directory surviving
  Fargate task replacement; controller SIGKILL/restart recovery; and scoped
  full-root cleanup plus instance-stack deletion while the other game stayed up.
- A separate AWS environment was completely destroyed after the first lifecycle
  run. Unexecuted CREATE plans were replanned and destroyed without allocating
  resources. The remaining test server has scheduled environment cleanup.
- Fake SDK tests exercise lost submission replies across intent changes, expired
  game/cleanup task history, explicit cleanup retries, ownership/API failures,
  and refusal to replace missing stacks or bypass unverified data cleanup.

These checks cover FTEQW and Xonotic, not every engine/provider combination.
The user has tested a vkQuake client against FTEQW; the public AWS endpoint
still needs their client/play session. Discord validation uses signed local
requests; live command registration has not been performed.

The ordinary checks are `make test` and `make check`. Real provider tests require
installed engines/assets or cloud credentials and remain separate from those
checks. Record the exact revision, image, environment, and cleanup result when
reporting a live test.

For further engine acceptance, exercise each provider with two
independent instances: distinct configuration/credentials/data/endpoints,
readiness, console/logs, controller restart, stop/start with retained files, game
crashes, and scoped deletion. Confirm behavior with real clients and human/bot
activity where practical.

For AWS also verify lost submission responses, expired task history, intent
changes during provisioning, task replacement with retained full-root data,
cleanup failures/retries, and environment retirement. A passing SDK test is
useful evidence for recovery logic but does not establish live networking,
filesystem behavior, or playability. Stopped games retain shared storage,
assets, images, and logs until explicitly cleaned up or the environment is
destroyed.

See [the README](../README.md) for commands and [architecture](architecture.md)
for ownership, lifecycle, transport, and persistence boundaries.
