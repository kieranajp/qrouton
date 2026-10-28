import assert from "node:assert/strict";
import test from "node:test";
import { terminalRenderer } from "./terminal-renderer.js";

function harness({ enabled = true, failLoad = false } = {}) {
  const addons = [];
  const recorded = [];
  const loaded = [];
  const renderer = terminalRenderer({
    term: {
      loadAddon: (addon) => {
        loaded.push(addon);
        if (failLoad) throw new Error("no webgl2");
      },
    },
    enabled,
    create: () => {
      const addon = {
        disposed: false,
        listeners: /** @type {(() => void)[]} */ ([]),
        dispose() { addon.disposed = true; },
        onContextLoss(listener) { addon.listeners.push(listener); },
        lose() { addon.listeners.forEach((listener) => listener()); },
      };
      addons.push(addon);
      return addon;
    },
    watch: () => (reason) => recorded.push(reason),
  });
  return { renderer, addons, recorded, loaded };
}

// Breaks if a terminal already on WebGL takes a second context when shown again.
test("showing a shown terminal creates nothing", () => {
  const { renderer, addons, loaded } = harness();
  renderer.show();
  renderer.show();

  assert.equal(addons.length, 1);
  assert.deepEqual(loaded, addons);
});

// Breaks if a hidden terminal keeps its context, where WebKit can evict it.
test("hiding disposes the addon and records a release", () => {
  const { renderer, addons, recorded } = harness();
  renderer.show();
  renderer.hide();

  assert.ok(addons[0].disposed);
  assert.deepEqual(recorded, ["released"]);
});

// Breaks if a terminal shown again after hiding stays on the DOM renderer.
test("showing after a hide creates a fresh addon", () => {
  const { renderer, addons } = harness();
  renderer.show();
  renderer.hide();
  renderer.show();

  assert.equal(addons.length, 2);
  assert.ok(!addons[1].disposed);
});

// Breaks if a lost context is left in place, or if recovery creates a context
// at once and evicts another terminal's.
test("a loss falls back until the terminal is hidden and shown again", () => {
  const { renderer, addons, recorded } = harness();
  renderer.show();
  addons[0].lose();
  renderer.show();

  assert.ok(addons[0].disposed);
  assert.equal(addons.length, 1);
  assert.deepEqual(recorded, ["fallback"]);

  renderer.hide();
  renderer.show();
  assert.equal(addons.length, 2);
  assert.deepEqual(recorded, ["fallback"]);
});

// Breaks if a platform kept on the DOM renderer ever creates a WebGL context.
test("a disabled renderer never creates", () => {
  const { renderer, addons } = harness({ enabled: false });
  renderer.show();
  renderer.hide();
  renderer.show();

  assert.equal(addons.length, 0);
});

// Breaks if tearing down a terminal on WebGL leaves its addon or its log entry behind.
test("disposing a shown terminal disposes the addon and records it", () => {
  const { renderer, addons, recorded } = harness();
  renderer.show();
  renderer.dispose();

  assert.ok(addons[0].disposed);
  assert.deepEqual(recorded, ["disposed"]);
});

// Breaks if an addon that failed to load stays registered on the terminal, piling
// up one more on every show, or is logged as a live context.
test("an addon that fails to load is disposed and leaves the terminal on DOM", (t) => {
  t.mock.method(console, "warn", () => {});
  const { renderer, addons, recorded } = harness({ failLoad: true });
  renderer.show();
  renderer.hide();

  assert.ok(addons[0].disposed);
  assert.deepEqual(recorded, []);
});
