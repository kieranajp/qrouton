import assert from "node:assert/strict";
import test from "node:test";
import { focusedIn, focusIn, storedWidth, widthKey } from "./layout.js";

// Two sessions sharing one key is one splitter drag applied to both windows.
test("each session stores its width under its own key", () => {
  assert.notEqual(widthKey("octopus"), widthKey("webhook"));
});

test("a session with no stored width falls back to untouched", () => {
  const stored = { [widthKey("octopus")]: "420" };
  const read = (key) => stored[key] ?? null;
  assert.equal(storedWidth(read, "octopus"), 420);
  assert.equal(storedWidth(read, "webhook"), 0);
});

test("unusable stored text reads as untouched rather than NaN", () => {
  assert.equal(
    storedWidth(() => "wide", "octopus"),
    0,
  );
});

// Switching sessions and back has to land on the tab you left up.
test("the focused tab map keeps one session's selection when another changes", () => {
  let focus = focusIn({}, "octopus", "window-3");
  focus = focusIn(focus, "webhook", "window-7");
  assert.equal(focusedIn(focus, "octopus"), "window-3");
  assert.equal(focusedIn(focus, "webhook"), "window-7");
  assert.equal(focusedIn(focus, "never-opened"), "");
});
