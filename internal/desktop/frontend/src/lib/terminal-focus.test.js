import assert from "node:assert/strict";
import test from "node:test";
import {
  consumeTerminalFocus,
  createTerminalActivation,
  focusGenerationIn,
  focusPendingIn,
  focusTerminal,
} from "./terminal-focus.js";

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

test("revealing a hidden terminal refits and then redraws its retained rows", () => {
  const queue = frames();
  const calls = [];
  const activation = createTerminalActivation({
    ...queue,
    refit() {
      calls.push("refit");
    },
    redraw() {
      calls.push("redraw");
    },
    focus() {},
    handled() {},
  });

  activation.update(true, 0, false);
  queue.flush();
  assert.deepEqual(calls, ["refit", "redraw"]);
});

test("each user terminal choice increments only that terminal's focus generation", () => {
  let generations = focusTerminal({}, "window-3");
  generations = focusTerminal(generations, "window-7");
  generations = focusTerminal(generations, "window-3");
  assert.equal(focusGenerationIn(generations, "window-3"), 2);
  assert.equal(focusGenerationIn(generations, "window-7"), 1);
});

test("an unrequested terminal is never pending, so activation alone takes no keyboard", () => {
  assert.equal(focusGenerationIn({}, "window-3"), 0);
  assert.equal(focusPendingIn({}, "window-3"), false);
});

test("focus requested before a terminal mounts stays pending and does not replay once handled", () => {
  let generations = focusTerminal({}, "window-3");
  assert.equal(focusPendingIn(generations, "window-3"), true);

  generations = consumeTerminalFocus(generations, "window-3", 1);
  assert.equal(focusPendingIn(generations, "window-3"), false);
  assert.equal(consumeTerminalFocus(generations, "window-3", 0), generations);
});
