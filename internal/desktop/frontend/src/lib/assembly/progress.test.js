import assert from "node:assert/strict";
import test from "node:test";
import { record } from "./progress.js";

const SVC = "org/svc";
const WEB = "org/web";

const advance = (repo, percent) => ({
  step: "mirror",
  status: "advanced",
  repo,
  phase: "Receiving objects",
  percent,
});

test("repeated advances for one step and repository fold to one row", () => {
  let rows = record([], { step: "mirror", status: "started", repo: SVC });
  for (let percent = 10; percent <= 90; percent += 10) rows = record(rows, advance(SVC, percent));
  assert.equal(rows.length, 2);
  assert.deepEqual([rows[1].state, rows[1].percent, rows[1].detail], [
    "running",
    90,
    "Receiving objects",
  ]);
});

test("a second repository's advance is a row of its own", () => {
  let rows = record([], advance(SVC, 20));
  rows = record(rows, advance(WEB, 5));
  assert.equal(rows.length, 2);
  assert.deepEqual(rows.map((row) => row.repo), ["org/svc", "org/web"]);
});

test("an outcome is never folded away", () => {
  let rows = record([], advance(SVC, 50));
  rows = record(rows, { step: "mirror", status: "completed", repo: SVC });
  assert.equal(rows.length, 2);
  assert.deepEqual([rows[1].state, rows[1].percent], ["done", undefined]);
});

test("a failure survives a later advance, and says what went wrong", () => {
  let rows = record([], { step: "mirror", status: "failed", repo: SVC, error: "no such host" });
  rows = record(rows, advance(SVC, 5));
  assert.deepEqual(rows.map((row) => row.state), ["failed", "running"]);
  assert.equal(rows[0].detail, "no such host");
});

test("a step with no repository labels itself and folds independently", () => {
  let rows = record([], { step: "scaffold", status: "started" });
  rows = record(rows, { step: "manifest", status: "completed" });
  assert.deepEqual(rows.map((row) => row.label), ["scaffold", "manifest"]);
});

test("recording leaves the rows it was handed alone", () => {
  const rows = record([], advance(SVC, 10));
  record(rows, advance(SVC, 20));
  assert.equal(rows.length, 1);
  assert.equal(rows[0].percent, 10);
});

test("an unrecognised status draws as pending rather than blank", () => {
  assert.equal(record([], { step: "mirror", status: "queued", repo: SVC })[0].state, "pending");
});
