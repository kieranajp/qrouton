import assert from "node:assert/strict";
import test from "node:test";
import { ICONS } from "./icons.js";

const NAMES = [
  "agent",
  "back",
  "bin",
  "branch",
  "close",
  "command",
  "copy",
  "disclosure-closed",
  "disclosure-open",
  "document",
  "down",
  "editing",
  "excluded",
  "failed",
  "forward",
  "menu",
  "new",
  "overflow",
  "pending",
  "reference",
  "refresh",
  "running",
  "search",
  "session",
  "settings",
  "shell",
  "submenu",
  "succeeded",
  "tree",
  "up",
  "waiting",
  "worktree",
];

test("the set holds exactly the Mitre icons", () => {
  assert.deepEqual(Object.keys(ICONS).sort(), [...NAMES].sort());
});

test("every icon draws something", () => {
  for (const [name, { stroke, fill }] of Object.entries(ICONS)) {
    assert.ok(stroke || fill, `${name} has no path`);
  }
});

test("no icon bends a line", () => {
  for (const [name, { stroke, fill }] of Object.entries(ICONS)) {
    for (const path of [stroke, fill].filter(Boolean)) {
      assert.doesNotMatch(path, /[cCsSqQtTaA]/, `${name} has a curve`);
    }
  }
});
