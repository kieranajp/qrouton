import assert from "node:assert/strict";
import test from "node:test";
import { menuHeight, place } from "./menu.js";

const VIEWPORT = { width: 1440, height: 900 };
const one = [{ label: "Clean up…" }];
const two = [{ label: "Reveal in Finder" }, "-", { label: "Clean up…" }];

test("the menu stands taller for every item it has to draw", () => {
  assert.ok(menuHeight(one) > menuHeight([]));
  assert.ok(menuHeight(two) > menuHeight(one));
});

test("a rule counts for less than an item, and an empty menu for its padding alone", () => {
  const rule = menuHeight(["-"]) - menuHeight([]);
  const item = menuHeight(one) - menuHeight([]);
  assert.ok(rule > 0 && rule < item);
  assert.ok(menuHeight([]) > 0);
});

// A menu is read downwards from the pointer, so it only flips when it has to.
test("a pointer with room below hangs the menu down and to the right of it", () => {
  const size = { width: 190, height: menuHeight(one) };
  const at = place({ x: 120, y: 300 }, size, VIEWPORT);
  assert.deepEqual(at, { left: 120, top: 300 });
});

test("a pointer near the bottom flips the menu up rather than off the viewport", () => {
  const size = { width: 190, height: menuHeight(one) };
  const at = place({ x: 120, y: VIEWPORT.height - 10 }, size, VIEWPORT);
  assert.ok(at.top < VIEWPORT.height - 10);
  assert.ok(at.top + size.height <= VIEWPORT.height);
});

// The taller menu overflows at a pointer position the shorter one still fits at.
test("a taller menu flips up where a shorter one still hangs down", () => {
  const y = VIEWPORT.height - menuHeight(one) - 20;
  const short = place({ x: 120, y }, { width: 190, height: menuHeight(one) }, VIEWPORT);
  const tall = place({ x: 120, y }, { width: 190, height: menuHeight(two) }, VIEWPORT);
  assert.equal(short.top, y);
  assert.ok(tall.top < y);
});

test("a pointer near the right edge pulls the menu back inside", () => {
  const size = { width: 190, height: menuHeight(one) };
  const at = place({ x: VIEWPORT.width - 20, y: 300 }, size, VIEWPORT);
  assert.ok(at.left + size.width <= VIEWPORT.width);
});

// A negative anchor draws the menu off the top or left of the window, where
// there is nothing to scroll it back into view.
test("the anchor never lands at a negative coordinate", () => {
  const size = { width: 190, height: menuHeight(two) };
  for (const viewport of [VIEWPORT, { width: 100, height: 60 }]) {
    for (const point of [
      { x: 0, y: 0 },
      { x: viewport.width, y: viewport.height },
      { x: 5, y: 5 },
    ]) {
      const at = place(point, size, viewport);
      assert.ok(at.left >= 0, `left ${at.left}`);
      assert.ok(at.top >= 0, `top ${at.top}`);
    }
  }
});
