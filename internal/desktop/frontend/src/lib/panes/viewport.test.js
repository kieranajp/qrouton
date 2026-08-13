import assert from "node:assert/strict";
import { test } from "node:test";
import { createViewportController, nextViewportSequence, normalizeIntervals } from "./viewport.js";

test("intervals sort and merge overlaps and adjacent blocks", () => {
  assert.deepEqual(
    normalizeIntervals([
      { line: 12, to: 14 },
      { line: 3, to: 5 },
      { line: 5, to: 9 },
      { line: 10, to: 10 },
      { line: 20, to: 21 },
    ]),
    [
      { line: 3, to: 10 },
      { line: 12, to: 14 },
      { line: 20, to: 21 },
    ],
  );
});

test("intervals reject missing, non-integral, and descending bounds", () => {
  for (const interval of [
    { line: 0, to: 1 },
    { line: 2, to: 1 },
    { line: 1.5, to: 2 },
    { line: 1, to: undefined },
  ]) {
    assert.throws(() => normalizeIntervals([interval]), /invalid source interval/);
  }
});

test("a window sequence remains monotonic across controller mounts", () => {
  const id = `window-${Math.random()}`;
  assert.equal(nextViewportSequence(id), 1);
  assert.equal(nextViewportSequence(id), 2);
});

test("controller coalesces frames and suppresses unchanged reports", () => {
  const reports = [];
  const frames = [];
  const listeners = new Map();
  const root = {
    addEventListener: (name, call) => listeners.set(name, call),
    removeEventListener: () => {},
    getBoundingClientRect: () => ({ top: 0, bottom: 100, width: 100, height: 100 }),
  };
  const block = {
    dataset: { line: "3", lineEnd: "5" },
    getBoundingClientRect: () => ({ top: 20, bottom: 60, width: 100, height: 40 }),
  };
  const content = {
    addEventListener: () => {},
    removeEventListener: () => {},
    querySelectorAll: () => [block],
  };
  const controller = createViewportController({
    root: /** @type {HTMLElement} */ (/** @type {unknown} */ (root)),
    content: /** @type {HTMLElement} */ (/** @type {unknown} */ (content)),
    selected: true,
    report: (value) => reports.push(value),
    requestFrame: (call) => (frames.push(call), frames.length),
    cancelFrame: () => {},
    resizeObserver: undefined,
    view: undefined,
    fonts: undefined,
  });
  controller.schedule();
  assert.equal(frames.length, 1);
  frames.shift()();
  assert.deepEqual(reports, [
    { seq: 1, available: true, selected: true, intervals: [{ line: 3, to: 5 }] },
  ]);
  listeners.get("scroll")();
  frames.shift()();
  assert.equal(reports.length, 1);
  controller.setSelected(false);
  assert.deepEqual(reports.at(-1), { seq: 2, available: false, selected: false, intervals: [] });
  controller.destroy();
  assert.equal(reports.length, 2);
});

function controllerHarness(selected = true) {
  const reports = [];
  const frames = new Map();
  const listeners = new Map();
  let nextFrame = 0;
  let resize;
  let reveals = 0;
  let targetRect = { top: 180, bottom: 220, width: 100, height: 40 };
  const root = {
    addEventListener: (name, call) => listeners.set(name, call),
    removeEventListener: () => {},
    getBoundingClientRect: () => ({ top: 0, bottom: 100, width: 100, height: 100 }),
  };
  const target = {
    dataset: { line: "8", lineEnd: "11" },
    getBoundingClientRect: () => targetRect,
    scrollIntoView: () => {
      reveals++;
      targetRect = { top: 30, bottom: 70, width: 100, height: 40 };
    },
  };
  const content = {
    addEventListener: () => {},
    removeEventListener: () => {},
    querySelectorAll: () => [target],
  };
  class Observer {
    constructor(call) {
      resize = call;
    }
    observe() {}
    unobserve() {}
    disconnect() {}
  }
  const controller = createViewportController({
    root: /** @type {HTMLElement} */ (/** @type {unknown} */ (root)),
    content: /** @type {HTMLElement} */ (/** @type {unknown} */ (content)),
    blocks: [/** @type {HTMLElement} */ (/** @type {unknown} */ (target))],
    target: /** @type {HTMLElement} */ (/** @type {unknown} */ (target)),
    span: { line: 9, to: 9 },
    selected,
    report: (value) => reports.push(value),
    requestFrame: (call) => {
      const id = ++nextFrame;
      frames.set(id, call);
      return id;
    },
    cancelFrame: (id) => frames.delete(id),
    resizeObserver: /** @type {typeof ResizeObserver} */ (Observer),
    view: undefined,
    fonts: undefined,
  });
  return {
    controller,
    reports,
    revealCount: () => reveals,
    moveTarget: (top, bottom) => (targetRect = { top, bottom, width: 100, height: bottom - top }),
    resize: () => resize(),
    scroll: () => listeners.get("scroll")(),
    flush: () => {
      const pending = [...frames.values()];
      frames.clear();
      pending.forEach((call) => call(0));
    },
  };
}

test("activation reveals the requested block after layout and reports its full interval", () => {
  const harness = controllerHarness();
  assert.equal(harness.revealCount(), 0);
  harness.flush();
  assert.equal(harness.revealCount(), 1);
  assert.deepEqual(harness.reports.at(-1), {
    seq: 1,
    available: true,
    selected: true,
    intervals: [{ line: 8, to: 11 }],
  });
});

test("layout recovers a previously visible target but a later scroll remains authoritative", () => {
  const harness = controllerHarness();
  harness.flush();
  harness.moveTarget(140, 180);
  harness.resize();
  harness.flush();
  assert.equal(harness.revealCount(), 2);

  harness.moveTarget(150, 190);
  harness.scroll();
  harness.flush();
  assert.deepEqual(harness.reports.at(-1).intervals, []);
  harness.resize();
  harness.flush();
  assert.equal(harness.revealCount(), 2);
});

test("a scroll event cancels a queued layout recovery", () => {
  const harness = controllerHarness();
  harness.flush();
  harness.moveTarget(140, 180);
  harness.resize();
  harness.scroll();
  harness.flush();
  assert.equal(harness.revealCount(), 1);
  assert.deepEqual(harness.reports.at(-1).intervals, []);
});

test("deactivation cancels a scheduled reveal and publishes unavailable state", () => {
  const harness = controllerHarness();
  harness.controller.setSelected(false);
  harness.flush();
  assert.equal(harness.revealCount(), 0);
  assert.deepEqual(harness.reports.at(-1), {
    seq: 1,
    available: false,
    selected: false,
    intervals: [],
  });
});
