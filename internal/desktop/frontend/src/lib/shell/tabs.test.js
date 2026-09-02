import assert from "node:assert/strict";
import test from "node:test";
import { dominantStatus, dropIndex, split } from "./tabs.js";

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

test("hidden tabs surface the state that most needs attention", () => {
  assert.equal(
    dominantStatus([{ status: "running" }, { status: "failed" }, { status: "waiting" }]),
    "waiting",
  );
  assert.equal(dominantStatus([{ status: "succeeded" }, { status: "failed" }]), "failed");
  assert.equal(dominantStatus([{}, { status: "succeeded" }]), "succeeded");
  assert.equal(dominantStatus([{}, {}]), "");
});

const entries = (...indexes) => indexes.map((index) => ({ index }));

const move = (list, from, to) => {
  const out = [...list];
  out.splice(to, 0, ...out.splice(from, 1));
  return out;
};

test("a drag across a fully drawn strip lands where a plain splice would", () => {
  const all = entries(0, 1, 2, 3, 4);
  assert.deepEqual(move(strip, 0, dropIndex(all, 0, 2)), [
    "plan",
    "shell 2",
    "shell",
    "shell 3",
    "research",
  ]);
  assert.deepEqual(move(strip, 3, dropIndex(all, 3, 1)), [
    "shell",
    "shell 3",
    "plan",
    "shell 2",
    "research",
  ]);
  assert.deepEqual(move(strip, 4, dropIndex(all, 4, 0)), [
    "research",
    "shell",
    "plan",
    "shell 2",
    "shell 3",
  ]);
});

test("a tab dropped on itself stays where it is", () => {
  assert.equal(dropIndex(entries(0, 1, 2, 3, 4), 2, 2), 2);
});

// The rightmost drawn tab is not the rightmost tab, so a drop on it has to name
// the place beside that tab rather than the drawn position it was let go over.
test("a drop on the rightmost drawn tab follows it through the whole strip", () => {
  assert.equal(dropIndex(entries(0, 1, 4), 0, 4), 4);
  assert.deepEqual(move(strip, 0, 4), ["plan", "shell 2", "shell 3", "research", "shell"]);
});

// The shape the strip actually draws when it overflows: a contiguous run, then
// the selected tab, then everything else waiting in the menu to their right.
test("a drop on the rightmost drawn tab stops short of the tabs hidden past it", () => {
  const longer = [...strip, "shell 4"];
  assert.equal(dropIndex(entries(0, 1, 3), 0, 3), 3);
  assert.deepEqual(move(longer, 0, 3), [
    "plan",
    "shell 2",
    "shell 3",
    "shell",
    "research",
    "shell 4",
  ]);
});

test("a leftward drop stops beside its new neighbour, not past the hidden tabs", () => {
  assert.equal(dropIndex(entries(0, 2, 4), 4, 2), 1);
  assert.deepEqual(move(strip, 4, 1), ["shell", "research", "plan", "shell 2", "shell 3"]);
});

test("a drop on the leftmost drawn tab goes to the front of the whole strip", () => {
  assert.equal(dropIndex(entries(1, 3, 4), 4, 1), 0);
});

test("a tab the strip never drew cannot be dragged anywhere", () => {
  assert.equal(dropIndex(entries(0, 1), 3, 1), 3);
});
