import assert from "node:assert/strict";
import test from "node:test";
import { isNarrow } from "./scale.js";

test("the narrow layout starts below 420px at 100%", () => {
  assert.equal(isNarrow(419, 100), true);
  assert.equal(isNarrow(420, 100), false);
});

test("the narrow layout starts below 630px at 150%", () => {
  assert.equal(isNarrow(629, 150), true);
  assert.equal(isNarrow(630, 150), false);
  assert.equal(isNarrow(500, 150), true);
  assert.equal(isNarrow(500, 100), false);
});
