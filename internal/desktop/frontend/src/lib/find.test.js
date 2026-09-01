import assert from "node:assert/strict";
import test from "node:test";
import { createFindAdapter, findShortcut } from "./find.js";

test("command-f and control-f open document find", () => {
  assert.equal(findShortcut({ key: "f", metaKey: true }), true);
  assert.equal(findShortcut({ key: "F", ctrlKey: true }), true);
});

test("document find leaves modified chords and other keys alone", () => {
  assert.equal(findShortcut({ key: "f" }), false);
  assert.equal(findShortcut({ key: "f", metaKey: true, shiftKey: true }), false);
  assert.equal(findShortcut({ key: "f", ctrlKey: true, altKey: true }), false);
  assert.equal(findShortcut({ key: "g", metaKey: true }), false);
  assert.equal(findShortcut(undefined), false);
});

test("find adapters refresh at the first match and wrap navigation", async () => {
  const activated = [];
  let resets = 0;
  const adapter = createFindAdapter({
    search(query) {
      assert.equal(query, "Needle");
      return ["first", "second", "third"];
    },
    activate(matches, index) {
      activated.push(index < 0 ? null : matches[index]);
    },
    reset() {
      resets += 1;
    },
  });

  assert.deepEqual(await adapter.refresh("Needle"), { count: 3, current: 0 });
  assert.deepEqual(await adapter.move(-1), { count: 3, current: 2 });
  assert.deepEqual(await adapter.move(1), { count: 3, current: 0 });
  assert.deepEqual(activated, ["first", "third", "first"]);
  assert.equal(resets, 1);
});

test("clearing a find adapter removes its matches and resets navigation", async () => {
  const activated = [];
  let resets = 0;
  const adapter = createFindAdapter({
    search: () => ["match"],
    activate(_matches, index) {
      activated.push(index);
    },
    reset: () => {
      resets += 1;
    },
  });

  await adapter.refresh("match");
  await adapter.clear();

  assert.deepEqual(await adapter.move(1), { count: 0, current: -1 });
  assert.deepEqual(activated, [0, -1]);
  assert.equal(resets, 2);
});
