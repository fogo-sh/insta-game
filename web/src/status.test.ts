import type { Action, Desired, Phase, ServerStatus } from "./types.ts";
import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import { serverState, stateLabel, serverActions } from "./status.ts";

test("start, prepare, ready, stop, retain data, restart, remove", () => {
  const server: ServerStatus = { desired: "running", phase: "queued", pending: true };
  assert.equal(serverState(server, "stopped"), "waking");
  server.phase = "preparing";
  assert.equal(serverState(server, "running"), "preparing");
  Object.assign(server, { phase: "running", pending: false });
  assert.equal(serverState(server), "running");
  server.desired = "stopped";
  assert.equal(serverState(server), "stopping");
  Object.assign(server, { phase: "stopped" });
  assert.equal(serverState(server), "stopped");
  assert(serverActions(server).some((a) => a.action === "start"));
  Object.assign(server, { desired: "running", phase: "queued" });
  assert.equal(serverState(server), "starting");
  server.desired = "deleted";
  assert.equal(serverState(server), "deleting");
  server.phase = "deleted";
  assert.equal(serverState(server), "deleted");
  assert.deepEqual(serverActions(server), []);
});

test("failed stop and delete stay visible as failures, with operation-specific retries", () => {
  for (const [desired, action] of [
    ["running", "start"],
    ["stopped", "stop"],
    ["deleted", "remove"],
  ] as [Desired, Action][]) {
    const server: ServerStatus = { desired, phase: "running", error: "provider unavailable" };
    assert.equal(stateLabel(serverState(server)), "Failed");
    assert(serverActions(server).some((a) => a.action === action && a.label.startsWith("Retry")));
  }
});

test("pending stop/remove does not become starting-host, and cannot restart early", () => {
  for (const desired of ["stopped", "deleted"] as Desired[]) {
    const server: ServerStatus = { desired, phase: "queued", pending: true };
    assert.equal(serverState(server, "stopped"), desired === "stopped" ? "stopping" : "deleting");
    assert(!serverActions(server).some((a) => a.action === "start" && !a.disabled));
  }
});

test("controller and fleet use the same labels, with no host override for terminal states", () => {
  for (const phase of ["preparing", "starting", "running", "stopped", "failed"] as Phase[]) {
    const desired = phase === "stopped" ? "stopped" : "running";
    assert.equal(
      serverState({ desired, phase }),
      serverState({ desired, phase, pending: false }, "running"),
    );
  }
  assert.equal(
    serverState({ desired: "running", phase: "failed", pending: false }, "stopped"),
    "failed",
  );
  assert.equal(stateLabel("unrecognized"), "Checking...");
});

test("lobby queued actions override historical snapshot intent until acknowledged", () => {
  const server: ServerStatus = {
    desired: "running",
    phase: "running",
    pending: true,
    action: "stop",
  };
  assert.equal(serverState(server, "stopped"), "stopping");
  assert(serverActions(server).find((a) => a.action === "start")?.disabled);
  Object.assign(server, { desired: "stopped", phase: "failed", action: "start" });
  assert.equal(serverState(server, "stopped"), "waking");
  assert(serverActions(server).some((a) => a.action === "stop"));
  Object.assign(server, { pending: false, requestError: "Request expired" });
  assert.equal(serverState(server), "failed");
  assert(serverActions(server).some((a) => a.action === "start" && a.label === "Retry"));
});
