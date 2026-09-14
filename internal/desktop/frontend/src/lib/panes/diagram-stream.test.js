import assert from "node:assert/strict";
import { test } from "node:test";
import { createDiagramStream } from "./diagram-stream.js";

test("late events and replies cannot overwrite a newer document", async () => {
  const pending = [];
  const applied = [];
  const stream = createDiagramStream(
    (request) => new Promise((resolve) => pending.push({ request, resolve })),
    (results) => applied.push(...results),
  );
  const first = stream.draw();
  const second = stream.draw();
  const old = { request: pending[0].request, line: 3, svg: "old" };
  const fresh = { request: pending[1].request, line: 3, svg: "fresh" };
  pending[1].resolve([fresh]);
  await second;
  stream.receive([old]);
  pending[0].resolve([old]);
  await first;
  assert.deepEqual(applied, [fresh]);
  stream.destroy();
  stream.receive([fresh]);
  assert.deepEqual(applied, [fresh]);
});

test("remounted panes reject earlier requests on the same event channel", async () => {
  let earlier;
  const old = createDiagramStream(async (request) => { earlier = { request }; return []; }, () => {});
  await old.draw();
  old.destroy();
  const applied = [];
  const fresh = createDiagramStream(async () => [], (results) => applied.push(...results));
  await fresh.draw();
  fresh.receive([earlier]);
  assert.deepEqual(applied, []);
});
