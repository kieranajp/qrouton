import assert from "node:assert/strict";
import test from "node:test";
import { createTerminalActivation } from "./terminal-focus.js";

function frames() {
  let sequence = 0;
  const queued = new Map();
  return {
    frame(run) {
      queued.set(++sequence, run);
      return sequence;
    },
    cancelFrame(id) {
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

test("a pre-mount focus request runs once and an acknowledged remount does not replay it", () => {
  const queue = frames();
  let focused = 0;
  let handled = 0;
  const activation = createTerminalActivation({
    ...queue,
    refit() {},
    focus() {
      focused++;
    },
    handled(generation) {
      handled = generation;
    },
  });
  activation.update(true, 1, true);
  queue.flush();
  assert.equal(focused, 1);
  assert.equal(handled, 1);

  const remounted = createTerminalActivation({
    ...queue,
    refit() {},
    focus() {
      focused++;
    },
    handled() {},
  });
  remounted.update(true, 1, false);
  queue.flush();
  assert.equal(focused, 1);
});

test("destroy cancels queued refit and focus work", () => {
  const queue = frames();
  let calls = 0;
  const activation = createTerminalActivation({
    ...queue,
    refit() {
      calls++;
    },
    focus() {
      calls++;
    },
    handled() {
      calls++;
    },
  });
  activation.update(true, 1, true);
  assert.equal(queue.size, 2);
  activation.destroy();
  assert.equal(queue.size, 0);
  queue.flush();
  assert.equal(calls, 0);
});
