# Finish the Go migration

Status: proposed implementation design. The current PR implements Docker and Linux native processes;
this document changes the completion criteria. It does not claim AWS or native
process support exists yet.

## Outcome

Ship one `insta-game` executable with one controller, API, CLI, web UI, game
agent, and lifecycle policy. Support native local processes, local Docker, and
AWS. Remove the TypeScript launcher, separate sidecar executable, Python Pulumi
application, compatibility entrypoints, and deployment instructions for the old
runtime when their responsibilities have been replaced. Fly is outside this
migration.

The project owner confirmed that `local` means native game processes without
Docker. Docker on a local machine already works and is a distinct target.

## Responsibilities

The controller owns configuration snapshots, credentials, desired state,
readiness, idle/max-runtime policy, and durable instance identity. Providers own
resource allocation, observation, start, stop, and deletion. The agent owns one
game process, protocol queries, config, console, and streaming logs. It never
calls a cloud API or decides which infrastructure to shut down.

Keep the existing small provider boundary. Add provider selection to creation
and dispatch through configured provider instances. Persist the chosen provider
with the instance; a restart must not reinterpret it using a new default.
Provider credentials and bootstrap settings belong to operator configuration,
not instance creation requests.

Add durable provider resource references where needed. A task ARN or native
agent identity must survive a controller restart. Checkpoint allocation before
starting further work. Lost responses, retries, and controller crashes must not
create duplicate games. Validate ownership before changing or deleting a
resource; a name or a process ID alone is insufficient.

The CLI and UI call the same instance API regardless of target. Do not duplicate
lifecycle policy in CLI commands, AWS scripts, Discord handlers, or the agent.

## Native local provider

Run `insta-game agent` as a separately supervised process for each instance,
with its own working directory, writable data, config, credentials, and ports.
The game executable and game assets are explicitly supplied by the operator;
this does not promise arbitrary commercial game installation.

Controller exit must not accidentally kill running agents. On restart, verify
the agent identity and inspect its status before reconnecting. Avoid trusting a
reused PID. Stop the game and retain the instance directory; delete only the
verified instance directory. Define the initial platform support explicitly
before claiming cross-platform process supervision.

## Docker provider

Retain the working Docker implementation and its ownership checks. Update game
images to use `/usr/local/bin/insta-game agent` as their only entrypoint. Remove
the separate sidecar build and binary. Keep per-instance writable data and the
existing stop/start/delete contract.

Move the shared query code from `sidecar/protocol` into the Go implementation
and update imports. The final repository has no sidecar runtime directory.

## AWS provider

Use ECS Fargate, preserving the existing hosting choice while replacing its
singleton launcher model. Prefer independently managed tasks over copying the
old per-game service names. The controller's durable instance is authoritative;
it must not rely on a service silently replacing crashed games when the product
requires explicit retry.

Each instance needs an owned task definition/resource identity, an observable
task attempt, actual discovered connection endpoints, and isolated persistent
data. Start after stop launches a new task attempt for the same instance. Bind
retry identity to that attempt so a lost RunTask response can be reconciled
without launching a duplicate. Persist enough information to distinguish
stopped, never started, failed, and provisioning instances.

Fargate writable layers are ephemeral. Use persistent storage, with a separate
EFS access point/directory per instance, before advertising the same stop-keeps-
data contract as Docker. Deleting an access point alone does not remove its
files: design and test scoped data cleanup as part of deletion. Do not mark an
instance deleted while owned resources or data still need cleanup.

Game ports may be publicly reachable; the agent management port must use a
private network route or authenticated encrypted transport. The initial AWS
controller should have VPC connectivity. Resolve the controller's deployment
and access path as part of bootstrap, not by exposing bearer-authenticated HTTP
over the public internet. Public game endpoints can change after task restart;
return discovered endpoints and surface the change in the UI.

Use the AWS SDK for Go. Provision shared resources through an embedded
CloudFormation template driven by the binary: network, IAM, cluster, logs,
persistent storage, and controller deployment/access. Provide a reviewable
change-set path before apply. Keep shared bootstrap separate from individual
instance lifecycle operations. There is no Python/Pulumi prerequisite in the
finished flow. Bootstrap must also support inspecting and destroying resources
it owns, with explicit treatment of retained game data.

Do not delete or rename existing live cloud resources as a side effect of
removing their source definitions. Inventory the deployed stack and plan the
cutover and retirement explicitly.

## Other responsibilities to migrate or explicitly remove

- Discord commands currently live in TypeScript. Retain their supported
  start/stop/status behavior in Go against instance IDs, or explicitly agree to
  remove Discord. Do not leave a second launcher just for this integration.
- Config editors and the simple web UI already have Go-owned equivalents.
  Remove the old frontend and its generated bundles.
- Remove the agent's ECS idle callback once the controller enforces all policy.
- Remove token-query/CORS/HTML-log compatibility only after confirming the
  retained Go clients do not require those contracts.
- Rewrite Compose generation, image publishing, CI, Makefiles, and deployment
  documentation around the single executable. A leftover build job is still a
  dependency on the old runtime.
- Do not carry forward CDN/Lambda infrastructure solely to preserve the old
  shape of deployment. Keep only the hosting responsibilities the new product
  actually needs.

## Implementation sequence

1. Inventory existing deployed resources and supported user-facing behavior.
   Resolve AWS controller access/deployment. Keep the PR
   draft; its current Docker-only state does not meet this design.
2. Introduce explicit provider selection and durable resource/attempt identity.
   Preserve existing Docker behavior and ownership guarantees.
3. Implement the native provider and exercise two real Quake instances, including
   controller termination/restart, stop/start, and deletion.
4. Implement AWS bootstrap and Fargate lifecycle, persistent storage, endpoint
   discovery, and private agent access. Test allocation retry and cleanup failures
   before a live deployment.
5. Exercise the complete AWS flow in an isolated stack with two actual games.
6. Move or explicitly remove remaining integrations, switch all image
   entrypoints, delete superseded code and workflows, and update PR scope.
7. Run the final maintainability review against the complete migration.

Temporary coexistence while implementing is not the finished product. Do not
merge this migration with an old runtime left as a supported fallback.

## Acceptance

For each provider, demonstrate two independent same-game instances, playable
connections, separate configs/credentials/data, console/log access, controller
process restart and recovery, independent stop/start, retained data, and scoped
deletion. Confirm actual bots versus human idle behavior and explicit recovery
from game crashes. Exercise loss of provider access without interpreting it as
an empty or deleted server.

For AWS additionally verify task replacement/retry identity, image architecture,
private management access, public game endpoints, cleanup after partial failure,
and shared-stack retirement. Use vkQuake to connect to FTEQW where practical;
SDK mocks alone do not establish that the deployment works.

The final diff must remove the old launcher and sidecar executable, migrate or
remove Pulumi responsibilities, and contain no documentation or workflows that
require a second application stack. No AWS deployment or destructive retirement
has been performed as part of writing this design.

## AWS references

- [Fargate task storage](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/fargate-task-storage.html)
- [Persistent EFS storage and access points](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/efs-volumes.html)
- [Fargate task networking](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/fargate-task-networking.html)
