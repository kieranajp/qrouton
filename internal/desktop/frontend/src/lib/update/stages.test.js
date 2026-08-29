import assert from "node:assert/strict";
import { test } from "node:test";
import {
  CHECKING,
  DOWNLOADING,
  FAILED,
  INSTALLING,
  PROGRESS,
  READY,
  progressed,
  staged,
} from "./stages.js";

test("the gate opens on the check rather than on an empty line", () => {
  const stage = staged();
  assert.equal(stage.percent, 0);
  assert.equal(stage.failed, false);
  assert.ok(stage.label.length > 0);
});

test("a download reports its share of the bytes", () => {
  const stage = progressed(staged(), PROGRESS, { written: 25, total: 100 });
  assert.equal(stage.percent, 25);
  assert.equal(stage.failed, false);
});

// A server that sends no length is a bar that cannot move, not one that has
// gone back to the start.
test("a download with no total holds the width it had", () => {
  const quarter = progressed(staged(), PROGRESS, { written: 25, total: 100 });
  assert.equal(progressed(quarter, PROGRESS, { written: 40, total: 0 }).percent, 25);
  assert.equal(progressed(quarter, PROGRESS, undefined).percent, 25);
});

test("progress is clamped to the bar", () => {
  assert.equal(progressed(staged(), PROGRESS, { written: 200, total: 100 }).percent, 100);
  assert.equal(progressed(staged(), PROGRESS, { written: -5, total: 100 }).percent, 0);
});

// The stages after the download keep the width they arrived with, so the bar
// does not fall back while the artifact is being verified and swapped.
test("verifying and installing keep the download's width", () => {
  const done = progressed(staged(), PROGRESS, { written: 100, total: 100 });
  assert.equal(progressed(done, INSTALLING).percent, 100);
  assert.equal(progressed(progressed(done, INSTALLING), READY).percent, 100);
});

test("a failure is reported without ending the gate", () => {
  const failed = progressed(staged(), FAILED);
  assert.equal(failed.failed, true);
  // And a retry clears it, because the policy does keep trying.
  assert.equal(progressed(failed, CHECKING).failed, false);
  assert.equal(progressed(failed, DOWNLOADING).failed, false);
});

test("an event the gate has no line for changes nothing", () => {
  const stage = progressed(staged(), PROGRESS, { written: 50, total: 100 });
  assert.deepEqual(progressed(stage, "wails:updater:no-update"), stage);
});
