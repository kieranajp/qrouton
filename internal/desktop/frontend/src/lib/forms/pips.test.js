import assert from "node:assert/strict";
import test from "node:test";
import { pipCounter, pipStates } from "./pips.js";

test("the pips behind the active one are done and the ones ahead are not", () => {
  assert.deepEqual(pipStates(5, 0), ["on", "todo", "todo", "todo", "todo"]);
  assert.deepEqual(pipStates(5, 2), ["done", "done", "on", "todo", "todo"]);
  assert.deepEqual(pipStates(5, 4), ["done", "done", "done", "done", "on"]);
});

test("an active step outside the pips still lights one of them", () => {
  assert.deepEqual(pipStates(3, 9), ["done", "done", "on"]);
  assert.deepEqual(pipStates(3, -2), ["on", "todo", "todo"]);
  assert.deepEqual(pipStates(0, 0), []);
});

test("the counter reads from one, and clamps with the pips", () => {
  assert.equal(pipCounter(5, 0), "1 of 5");
  assert.equal(pipCounter(5, 4), "5 of 5");
  assert.equal(pipCounter(5, 9), "5 of 5");
});
