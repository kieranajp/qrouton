import assert from "node:assert/strict";
import test from "node:test";
import {
  MIN_AGENT,
  MIN_HUMAN,
  MAX_SIDEBAR,
  MIN_SIDEBAR,
  humanWidth,
  roomFor,
  selectedTab,
  sidebarWidth,
  sidebarWidthKey,
  storedSidebarWidth,
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

test("the sidebar keeps one persisted width across sessions", () => {
  const stored = { [sidebarWidthKey()]: "260" };
  assert.equal(storedSidebarWidth((key) => stored[key] ?? null), 260);
  assert.equal(sidebarWidth(260), 260);
  assert.equal(sidebarWidth(10), MIN_SIDEBAR);
  assert.equal(sidebarWidth(1000), MAX_SIDEBAR);
  assert.equal(sidebarWidth(0), 0);
});

const TABS = [{ id: "window-1" }, { id: "window-3" }, { id: "window-7" }];

test("the strip follows the selection Go published", () => {
  assert.equal(selectedTab(TABS, "window-7"), 2);
});

// The first shell opens without a selection, and there is a tab to draw.
test("no selection yet opens on the leftmost tab", () => {
  assert.equal(selectedTab(TABS, ""), 0);
  assert.equal(selectedTab([], ""), -1);
});

// Claiming tab 0 instead would show a selection Go never made.
test("a selection naming no open tab selects nothing", () => {
  assert.equal(selectedTab(TABS, "window-9"), -1);
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
