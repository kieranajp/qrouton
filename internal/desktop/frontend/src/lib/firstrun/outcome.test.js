import assert from "node:assert/strict";
import test from "node:test";
import { firstRunOutcome } from "./outcome.js";

test("a refusal naming a field marks it and says so in the footer", () => {
  const got = firstRunOutcome(undefined, new Error("root: cannot be empty"));
  assert.deepEqual(got.fields, { root: "cannot be empty" });
  assert.equal(got.status, "cannot be empty");
});

// A failed relaunch has no field to blame; the message names the log to read.
test("a refusal naming no field fills only the footer", () => {
  const message = "workbench never answered on its socket (see /tmp/x.log)";
  const got = firstRunOutcome(undefined, new Error(message));
  assert.deepEqual(got.fields, {});
  assert.equal(got.status, message);
});

// Both successes leave the screen alone: Go drops the gate, not the page.
test("a save that worked says nothing, whether or not it relaunches", () => {
  for (const result of [{ relaunching: false }, { relaunching: true }]) {
    const got = firstRunOutcome(result, undefined);
    assert.deepEqual(got.fields, {});
    assert.equal(got.status, "");
  }
});
