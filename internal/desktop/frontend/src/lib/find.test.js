import assert from "node:assert/strict";
import test from "node:test";
import { findShortcut } from "./find.js";

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
