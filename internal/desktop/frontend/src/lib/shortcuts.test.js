import assert from "node:assert/strict";
import test from "node:test";
import { NUMBERED, position, shortcut } from "./shortcuts.js";

const cmd = (key, extra = {}) => ({ key, metaKey: true, ...extra });

test("command and a digit name the rail row at that position", () => {
  assert.equal(position(cmd("1")), 1);
  assert.equal(position(cmd("9")), 9);
});

// Nothing wraps: a tenth session is click-only rather than sharing a digit.
test("zero and anything past the ninth row name no session", () => {
  assert.equal(position(cmd("0")), 0);
  assert.equal(position(cmd("a")), 0);
  assert.equal(position(cmd("")), 0);
});

// The terminal owns Control and Option, and shift-digit is punctuation.
test("a digit with any other modifier is the terminal's", () => {
  assert.equal(position({ key: "1" }), 0);
  assert.equal(position(cmd("1", { shiftKey: true })), 0);
  assert.equal(position(cmd("1", { altKey: true })), 0);
  assert.equal(position(cmd("1", { ctrlKey: true })), 0);
  assert.equal(position({ key: "1", ctrlKey: true }), 0);
});

test("position tolerates being handed nothing", () => {
  assert.equal(position(undefined), 0);
});

test("the first nine rows wear their shortcut and the rest wear none", () => {
  assert.equal(shortcut(0), "⌘1");
  assert.equal(shortcut(NUMBERED - 1), "⌘" + NUMBERED);
  assert.equal(shortcut(NUMBERED), "");
});
