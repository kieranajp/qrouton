import assert from "node:assert/strict";
import test from "node:test";
import { place } from "./menu.js";

const VIEWPORT = { width: 1440, height: 900 };
const SHORT = 60;
const TALL = 120;

test("a pointer beyond a resized viewport keeps the menu inside it", () => {
  const size = { width: 190, height: 180 };
  const viewport = { width: 540, height: 420 };
  const at = place({ x: 635, y: 475 }, size, viewport);
  assert.ok(at.left + size.width <= viewport.width);
  assert.ok(at.top + size.height <= viewport.height);
});

// A menu is read downwards from the pointer, so it only flips when it has to.
test("a pointer with room below hangs the menu down and to the right of it", () => {
  const size = { width: 190, height: SHORT };
  const at = place({ x: 120, y: 300 }, size, VIEWPORT);
  assert.deepEqual(at, { left: 120, top: 300 });
});

test("a pointer near the bottom flips the menu up rather than off the viewport", () => {
  const size = { width: 190, height: SHORT };
  const at = place({ x: 120, y: VIEWPORT.height - 10 }, size, VIEWPORT);
  assert.ok(at.top < VIEWPORT.height - 10);
  assert.ok(at.top + size.height <= VIEWPORT.height);
});

// The taller menu overflows at a pointer position the shorter one still fits at.
test("a taller menu flips up where a shorter one still hangs down", () => {
  const y = VIEWPORT.height - SHORT - 20;
  const short = place({ x: 120, y }, { width: 190, height: SHORT }, VIEWPORT);
  const tall = place({ x: 120, y }, { width: 190, height: TALL }, VIEWPORT);
  assert.equal(short.top, y);
  assert.ok(tall.top < y);
});

test("a pointer near the right edge pulls the menu back inside", () => {
  const size = { width: 190, height: SHORT };
  const at = place({ x: VIEWPORT.width - 20, y: 300 }, size, VIEWPORT);
  assert.ok(at.left + size.width <= VIEWPORT.width);
});

// A negative anchor draws the menu off the top or left of the window, where
// there is nothing to scroll it back into view.
test("the anchor never lands at a negative coordinate", () => {
  const size = { width: 190, height: TALL };
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

test("a scaled menu flips at the size it is drawn, not its base size", () => {
  const size = { width: 190, height: 100 };
  const point = { x: VIEWPORT.width - 250, y: 120 };
  assert.equal(place(point, size, VIEWPORT).left, point.x);
  assert.equal(place(point, size, VIEWPORT, 1.5).left, VIEWPORT.width - 285 - 8);
});
