import assert from "node:assert/strict";
import test from "node:test";
import { latestPerFrame } from "./frame.js";

function frames() {
  let sequence = 0;
  const queued = new Map();
  return {
    request(run) {
      const id = ++sequence;
      queued.set(id, run);
      return id;
    },
    cancel(id) {
      queued.delete(id);
    },
    flush() {
      const runs = [...queued.values()];
      queued.clear();
      runs.forEach((run) => run());
    },
    get size() {
      return queued.size;
    },
  };
}

test("a frame receives only the latest scheduled value", () => {
  const queue = frames();
  const values = [];
  const scheduled = latestPerFrame((value) => values.push(value), queue.request, queue.cancel);

  scheduled.schedule(320);
  scheduled.schedule(408);
  scheduled.schedule(512);

  assert.equal(queue.size, 1);
  queue.flush();
  assert.deepEqual(values, [512]);
});

test("work scheduled after a frame runs uses a new frame", () => {
  const queue = frames();
  const values = [];
  const scheduled = latestPerFrame((value) => values.push(value), queue.request, queue.cancel);

  scheduled.schedule("first");
  queue.flush();
  scheduled.schedule("second");

  assert.equal(queue.size, 1);
  queue.flush();
  assert.deepEqual(values, ["first", "second"]);
});

test("cancel drops the pending value and permits later work", () => {
  const queue = frames();
  const values = [];
  const scheduled = latestPerFrame((value) => values.push(value), queue.request, queue.cancel);

  scheduled.schedule("stale");
  scheduled.cancel();
  queue.flush();
  assert.deepEqual(values, []);

  scheduled.schedule("fresh");
  queue.flush();
  assert.deepEqual(values, ["fresh"]);
});
