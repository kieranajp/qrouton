import assert from "node:assert/strict";
import test from "node:test";
import {
  MIN_AGENT,
  MIN_HUMAN,
  consumeTerminalFocus,
  focusGenerationIn,
  focusPendingIn,
  focusTerminal,
  humanWidth,
  roomFor,
  selectedIn,
  selectIn,
  storedWidth,
  widthKey,
} from "./layout.js";

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
test("complete snapshots restore missed selections and keep sessions separate", () => {
  let selection = selectIn({}, "octopus", "window-3");
  selection = selectIn(selection, "webhook", "window-7");
  selection = selectIn(selection, "octopus", "window-5");
  assert.equal(selectedIn(selection, "octopus"), "window-5");
  assert.equal(selectedIn(selection, "webhook"), "window-7");
  assert.equal(selectedIn(selection, "never-opened"), "");
});

test("agent selection changes visibility without keyboard focus intent", () => {
  const selection = selectIn({}, "octopus", "window-3");
  assert.equal(selectedIn(selection, "octopus"), "window-3");
  assert.equal(focusGenerationIn({}, "window-3"), 0);
});

test("each user terminal choice increments only that terminal's focus generation", () => {
  let generations = focusTerminal({}, "window-3");
  generations = focusTerminal(generations, "window-7");
  generations = focusTerminal(generations, "window-3");
  assert.equal(focusGenerationIn(generations, "window-3"), 2);
  assert.equal(focusGenerationIn(generations, "window-7"), 1);
});

test("focus requested before a terminal mounts stays pending and does not replay once handled", () => {
  let generations = focusTerminal({}, "window-3");
  assert.equal(focusGenerationIn(generations, "window-3"), 1);
  assert.equal(focusPendingIn(generations, "window-3"), true);

  generations = consumeTerminalFocus(generations, "window-3", 1);
  assert.equal(focusPendingIn(generations, "window-3"), false);
  assert.equal(consumeTerminalFocus(generations, "window-3", 0), generations);
});

// The pane arithmetic below moved out of Session.svelte, where nothing could
// reach it: the splitter's own tests drive Splitter.svelte with handlers of
// their own and never see this clamp.

test("an unmeasured window imposes no limit", () => {
  assert.equal(roomFor(0, 0), Infinity);
  assert.equal(roomFor(0, 240), Infinity);
});

test("room is what is left once the rail and the agent's minimum are taken out", () => {
  assert.equal(roomFor(1400, 240), 1400 - 240 - MIN_AGENT);
});

// The divider stops rather than letting either pane become a strip, so the
// agent gives up its minimum before the shell does.
test("a window too narrow for both panes still leaves the shell its minimum", () => {
  assert.equal(roomFor(500, 240), MIN_HUMAN);
  assert.equal(roomFor(1, 0), MIN_HUMAN);
});

test("an untouched width stays untouched, so the pane keeps its own default", () => {
  assert.equal(humanWidth(0, 800), 0);
  assert.equal(humanWidth(0, Infinity), 0);
});

test("a width is clamped up to the minimum and down to the room there is", () => {
  assert.equal(humanWidth(100, 800), MIN_HUMAN);
  assert.equal(humanWidth(500, 800), 500);
  assert.equal(humanWidth(900, 800), 800);
});

test("the minimum wins over the room when a window cannot fit both", () => {
  assert.equal(humanWidth(1000, roomFor(500, 240)), MIN_HUMAN);
});
