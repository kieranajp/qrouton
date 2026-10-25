import assert from "node:assert/strict";
import test from "node:test";
import { ICONS } from "./icons.js";

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
