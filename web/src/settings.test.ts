import type { Game } from "./types.ts";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "vite-plus/test";
import { renderControl } from "./settings.ts";

const catalog: Game = JSON.parse(
  await readFile(new URL("../../docker-containers/openarena/game.json", import.meta.url), "utf8"),
);
assert(catalog.configEditor);
const controls = catalog.configEditor.controls;
const rotationControl = controls.find((control) => control.id === "maps");
assert(rotationControl);
const rotation = rotationControl.render;

test("OpenArena settings render a complete cyclic map rotation", () => {
  const lines = controls.flatMap((control) => renderControl(control.render, control.defaultValue));
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
