import assert from "node:assert/strict";
import test from "node:test";
import { debounced } from "./async.js";

function timers() {
  let sequence = 0;
  const queued = new Map();
  return {
    set(run) {
      const id = ++sequence;
      queued.set(id, run);
      return id;
    },
    clear(id) {
      queued.delete(id);
    },
    fire() {
      const runs = [...queued.values()];
      queued.clear();
      runs.forEach((run) => run());
    },
    get size() {
      return queued.size;
    },
  };
}

function deferred() {
  /** @type {any} */
  let settle;
  const answering = new Promise((resolve) => (settle = resolve));
  return { answering, settle };
}

const settled = () => new Promise((resolve) => setTimeout(resolve, 0));

test("a burst of values asks once, for the newest of them", async () => {
  const clock = timers();
  const asked = [];
  const landed = [];
  const run = debounced(5, async (value) => (asked.push(value), value), (answer) => landed.push(answer), clock);

  run.schedule("qro");
  run.schedule("qrou");
  run.schedule("qrout");

  assert.equal(clock.size, 1);
  clock.fire();
  await settled();
  assert.deepEqual(asked, ["qrout"]);
  assert.deepEqual(landed, ["qrout"]);
});

test("a slow answer to a superseded value never lands over the newer one", async () => {
  const clock = timers();
  const answers = new Map();
  const landed = [];
  const run = debounced(
    5,
    (value) => {
      const held = deferred();
      answers.set(value, held);
      return held.answering;
    },
    (answer) => landed.push(answer),
    clock,
  );

  run.schedule("old");
  clock.fire();
  run.schedule("new");
  clock.fire();

  answers.get("new").settle("feat/new");
  await settled();
  answers.get("old").settle("feat/old");
  await settled();

  assert.deepEqual(landed, ["feat/new"]);
});

test("cancel drops the answer to a call already with the bridge", async () => {
  const clock = timers();
  const held = deferred();
  const landed = [];
  const run = debounced(5, () => held.answering, (answer) => landed.push(answer), clock);

  run.schedule("typed");
  clock.fire();
  run.cancel();
  held.settle("feat/typed");
  await settled();

  assert.deepEqual(landed, []);
});

test("cancel before the timer fires asks nothing at all", async () => {
  const clock = timers();
  const asked = [];
  const run = debounced(5, async (value) => (asked.push(value), value), () => {}, clock);

  run.schedule("typed");
  run.cancel();
  clock.fire();
  await settled();

  assert.equal(clock.size, 0);
  assert.deepEqual(asked, []);
});
