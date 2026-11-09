import assert from "node:assert/strict";
import { test } from "node:test";
import { pinDocumentScroll } from "./document-scroll.js";

const fakeWindow = (scrollTop, scrollLeft) => {
  const writes = [];
  const el = {
    get scrollTop() { return scrollTop; },
    set scrollTop(v) { writes.push("top"); scrollTop = v; },
    get scrollLeft() { return scrollLeft; },
    set scrollLeft(v) { writes.push("left"); scrollLeft = v; },
  };
  let listener;
  const view = {
    document: { scrollingElement: el },
    addEventListener: (type, fn, opts) => { assert.equal(type, "scroll"); assert.equal(opts.passive, true); listener = fn; },
    removeEventListener: () => { listener = undefined; },
  };
  return { view: /** @type {Window & typeof globalThis} */ (/** @type {unknown} */ (view)), el, writes, fire: () => listener?.(), attached: () => !!listener };
};

// Regression: a stray reveal leaves the document scrolled past the layout.
test("a scroll event with non-zero offsets resets both to 0", () => {
  const w = fakeWindow(0, 0);
  pinDocumentScroll(w.view);
  w.el.scrollTop = 741;
  w.el.scrollLeft = 12;
  w.fire();
  assert.equal(w.el.scrollTop, 0);
  assert.equal(w.el.scrollLeft, 0);
});

// Regression: writing scroll offsets when nothing moved would feed scroll events back.
test("a zero state is left alone and detach stops the listener", () => {
  const w = fakeWindow(0, 0);
  const detach = pinDocumentScroll(w.view);
  w.fire();
  assert.deepEqual(w.writes, []);
  detach();
  assert.equal(w.attached(), false);
});
