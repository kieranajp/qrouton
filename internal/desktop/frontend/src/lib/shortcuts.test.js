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
  assert.equal(opensSettings(cmd(","), true), true);
  assert.equal(opensSettings({ key: ",", ctrlKey: true }, false), true);
});

test("comma with the other platform's modifier stays with the terminal", () => {
  assert.equal(opensSettings({ key: ",", ctrlKey: true }, true), false);
  assert.equal(opensSettings(cmd(","), false), false);
});

test("a comma the settings panel has no claim on", () => {
  assert.equal(opensSettings({ key: "," }, true), false);
  assert.equal(opensSettings(cmd(",", { ctrlKey: true }), true), false);
  assert.equal(opensSettings(cmd(",", { shiftKey: true }), true), false);
  assert.equal(opensSettings(cmd(",", { altKey: true }), true), false);
  assert.equal(opensSettings(cmd("."), true), false);
});

test("Command with =, +, - and 0 steps and resets the scale on macOS", () => {
  assert.equal(scaleAction(cmd("="), true), "in");
  assert.equal(scaleAction(cmd("+", { shiftKey: true }), true), "in");
  assert.equal(scaleAction(cmd("-"), true), "out");
  assert.equal(scaleAction(cmd("0"), true), "reset");
});

test("Control steps and resets the scale on Linux", () => {
  assert.equal(scaleAction({ key: "=", ctrlKey: true }, false), "in");
  assert.equal(scaleAction({ key: "+", ctrlKey: true, shiftKey: true }, false), "in");
  assert.equal(scaleAction({ key: "-", ctrlKey: true }, false), "out");
  assert.equal(scaleAction({ key: "0", ctrlKey: true }, false), "reset");
});

// Control-minus is readline's undo on macOS, so only Command scales there.
test("the other platform's modifier, Option, both modifiers and other keys leave the scale alone", () => {
  assert.equal(scaleAction({ key: "-", ctrlKey: true }, true), "");
  assert.equal(scaleAction(cmd("="), false), "");
  assert.equal(scaleAction(cmd("=", { altKey: true }), true), "");
  assert.equal(scaleAction({ key: "=", ctrlKey: true, altKey: true }, false), "");
  assert.equal(scaleAction(cmd("=", { ctrlKey: true }), true), "");
  assert.equal(scaleAction({ key: "=", ctrlKey: true, metaKey: true }, false), "");
  assert.equal(scaleAction(cmd("1"), true), "");
  assert.equal(scaleAction({ key: "=" }, true), "");
});

test("switcher requires the exact platform chord and reserves repeats", async () => {
  const { opensSwitcher } = await import("./shortcuts.js");
  for (const mac of [true, false]) {
    const chord = { key: "K", metaKey: mac, ctrlKey: !mac };
    assert.ok(opensSwitcher(chord, mac));
    assert.ok(opensSwitcher({ ...chord, repeat: true }, mac));
    assert.ok(!opensSwitcher(chord, !mac));
    for (const extra of [{ altKey: true }, { shiftKey: true }, { metaKey: true, ctrlKey: true }, { isComposing: true }, { keyCode: 229 }]) assert.ok(!opensSwitcher({ ...chord, ...extra }, mac));
  }
});
