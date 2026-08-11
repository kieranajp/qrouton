import assert from "node:assert/strict";
import test from "node:test";
import { split } from "./tabs.js";

const strip = ["shell", "plan", "shell 2", "shell 3", "research"];

test("a strip with room for everything hides nothing", () => {
  const { shown, hidden } = split(strip, 0, strip.length);
  assert.deepEqual(
    shown.map((entry) => entry.tab),
    strip,
  );
  assert.deepEqual(hidden, []);
});

test("the tabs past capacity go to the menu, keeping their own indexes", () => {
  const { shown, hidden } = split(strip, 0, 3);
  assert.deepEqual(
    shown.map((entry) => entry.tab),
    ["shell", "plan"],
  );
  assert.deepEqual(
    hidden.map((entry) => entry.index),
    [2, 3, 4],
  );
});

// A click has to reach the window it named, not the one at that spot in the menu.
test("a hidden tab reports the index it has in the whole strip", () => {
  const { hidden } = split(strip, 0, 3);
  assert.equal(hidden[2].tab, "research");
  assert.equal(strip[hidden[2].index], "research");
});

test("the selected tab is drawn even when it sits past capacity", () => {
  const { shown, hidden } = split(strip, 4, 3);
  assert.deepEqual(
    shown.map((entry) => entry.tab),
    ["shell", "plan", "research"],
  );
  assert.ok(!hidden.some((entry) => entry.index === 4));
});

test("room for one tab is room for the selected one", () => {
  const { shown, hidden } = split(strip, 3, 1);
  assert.deepEqual(
    shown.map((entry) => entry.index),
    [3],
  );
  assert.equal(hidden.length, 4);
});
