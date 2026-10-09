import assert from "node:assert/strict";
import test from "node:test";
import { NUMBERED, opensSettings, position, rowAt, scaleAction, shortcut } from "./shortcuts.js";

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

// An off-by-one here sends every shortcut to the neighbouring session.
test("a shortcut names the row at that position, counting from one", () => {
  const rows = [{ slug: "kraken" }, { slug: "octopus" }, { slug: "webhook" }];
  assert.equal(rowAt(rows, cmd("1")).slug, "kraken");
  assert.equal(rowAt(rows, cmd("3")).slug, "webhook");
});

test("a shortcut past the last row, or no shortcut at all, names nothing", () => {
  const rows = [{ slug: "kraken" }];
  assert.equal(rowAt(rows, cmd("2")), undefined);
  assert.equal(rowAt(rows, cmd("0")), undefined);
  assert.equal(rowAt(rows, { key: "1" }), undefined);
  assert.equal(rowAt([], cmd("1")), undefined);
});

test("comma with the platform's own modifier asks for settings", () => {
  assert.equal(opensSettings(cmd(",")), true);
  assert.equal(opensSettings({ key: ",", ctrlKey: true }), true);
});

test("a comma the settings panel has no claim on", () => {
  assert.equal(opensSettings({ key: "," }), false);
  assert.equal(opensSettings(cmd(",", { ctrlKey: true })), false);
  assert.equal(opensSettings(cmd(",", { shiftKey: true })), false);
  assert.equal(opensSettings(cmd(",", { altKey: true })), false);
  assert.equal(opensSettings(cmd(".")), false);
  assert.equal(opensSettings(undefined), false);
});

test("Command with =, +, - and 0 steps and resets the scale on macOS", () => {
  assert.equal(scaleAction(cmd("="), false), "in");
  assert.equal(scaleAction(cmd("+", { shiftKey: true }), false), "in");
  assert.equal(scaleAction(cmd("-"), false), "out");
  assert.equal(scaleAction(cmd("0"), false), "reset");
});

test("Control steps and resets the scale on Linux", () => {
  assert.equal(scaleAction({ key: "=", ctrlKey: true }, true), "in");
  assert.equal(scaleAction({ key: "+", ctrlKey: true, shiftKey: true }, true), "in");
  assert.equal(scaleAction({ key: "-", ctrlKey: true }, true), "out");
  assert.equal(scaleAction({ key: "0", ctrlKey: true }, true), "reset");
});

// Control-minus is readline's undo on macOS, so only Command scales there.
test("the other platform's modifier, Option, both modifiers and other keys leave the scale alone", () => {
  assert.equal(scaleAction({ key: "-", ctrlKey: true }, false), "");
  assert.equal(scaleAction(cmd("="), true), "");
  assert.equal(scaleAction(cmd("=", { altKey: true }), false), "");
  assert.equal(scaleAction({ key: "=", ctrlKey: true, altKey: true }, true), "");
  assert.equal(scaleAction(cmd("=", { ctrlKey: true }), false), "");
  assert.equal(scaleAction({ key: "=", ctrlKey: true, metaKey: true }, true), "");
  assert.equal(scaleAction(cmd("1"), false), "");
  assert.equal(scaleAction({ key: "=" }, false), "");
  assert.equal(scaleAction(undefined, false), "");
});
