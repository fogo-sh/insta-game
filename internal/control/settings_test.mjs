import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { renderControl } from "./web/settings.js";

const catalog = JSON.parse(
  await readFile(new URL("../../docker-containers/openarena/game.json", import.meta.url)),
);
const controls = catalog.configEditor.controls;
const rotation = controls.find((control) => control.id === "maps").render;

test("OpenArena settings render a complete cyclic map rotation", () => {
  const lines = controls.flatMap((control) =>
    renderControl(control.render, control.defaultValue),
  );
  assert(lines.includes('set sv_hostname "Insta Game OpenArena"'));
  assert(lines.includes('set d1 "map oa_ctf4ish; set nextmap vstr d2"'));
  assert(lines.includes('set d4 "map oa_dm3; set nextmap vstr d1"'));
  assert(lines.includes('set nextmap "vstr d2"'));
  assert(lines.includes("map oa_ctf4ish"));
  assert(!lines.some((line) => line.includes("{{")));
});

test("one map loops to itself and an empty selection emits no rotation", () => {
  assert.deepEqual(renderControl(rotation, ["oa_dm5"]), [
    'set d1 "map oa_dm5; set nextmap vstr d1"',
    'set nextmap "vstr d1"',
    "",
    "map oa_dm5",
  ]);
  assert.deepEqual(renderControl(rotation, []), []);
});
