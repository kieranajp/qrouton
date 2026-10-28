import assert from "node:assert/strict";
import test from "node:test";
import { INSPECT_QUERY } from "./bridge/generated.js";
import { webglWatcher } from "./webgl-log.js";

function addonDouble() {
  /** @type {{listener?: () => void, disposed: boolean, onContextLoss: (listener: () => void) => {dispose: () => void}}} */
  const addon = {
    disposed: false,
    onContextLoss: (listener) => {
      addon.listener = listener;
      return { dispose: () => { addon.disposed = true; } };
    },
  };
  return addon;
}

function recorder(search) {
  const sent = [];
  const watch = webglWatcher(search, (...record) => sent.push(record));
  return { sent, watch };
}

// Breaks if a renderer's creation, context loss, restoration or disposal stops
// being reported, or the page-wide live count drifts from created minus disposed.
test("each renderer reports its lifecycle with the page's live count", () => {
  const { sent, watch } = recorder("?" + INSPECT_QUERY);
  const first = { host: new EventTarget(), addon: addonDouble() };
  const second = { host: new EventTarget(), addon: addonDouble() };

  const unwatchFirst = watch({ ...first, terminal: "term-1", pane: "conversation" });
  const unwatchSecond = watch({ ...second, terminal: "win-2", pane: "tab" });
  first.host.dispatchEvent(new Event("webglcontextlost"));
  first.host.dispatchEvent(new Event("webglcontextrestored"));
  second.addon.listener?.();
  unwatchFirst("disposed");
  unwatchSecond("disposed");

  assert.deepEqual(sent, [
    ["created", "term-1", "conversation", 1, 1],
    ["created", "win-2", "tab", 2, 2],
    ["lost", "term-1", "conversation", 2, 2],
    ["restored", "term-1", "conversation", 2, 2],
    ["unrestored", "win-2", "tab", 2, 2],
    ["disposed", "term-1", "conversation", 1, 2],
    ["disposed", "win-2", "tab", 0, 2],
  ]);
});

// Breaks if a disposed renderer keeps listening, so a later loss on a reused
// host would be blamed on a renderer that no longer exists.
test("a disposed renderer stops reporting", () => {
  const { sent, watch } = recorder("?" + INSPECT_QUERY);
  const renderer = { host: new EventTarget(), addon: addonDouble() };
  watch({ ...renderer, terminal: "term-1", pane: "conversation" })("disposed");
  renderer.host.dispatchEvent(new Event("webglcontextlost"));

  assert.equal(sent.length, 2);
  assert.ok(renderer.addon.disposed);
});

// Breaks if the page records without QROUTON_INSPECT.
test("nothing is recorded when the page is not inspecting", () => {
  const { sent, watch } = recorder("");
  const renderer = { host: new EventTarget(), addon: addonDouble() };
  const unwatch = watch({ ...renderer, terminal: "term-1", pane: "conversation" });
  renderer.host.dispatchEvent(new Event("webglcontextlost"));
  unwatch("disposed");

  assert.deepEqual(sent, []);
  assert.equal(renderer.addon.listener, undefined);
});

// Breaks if the host stops capturing: the context events fire on the canvas
// inside it and never bubble up, so a real loss would go unlogged.
test("the host listens for context events while capturing", () => {
  const captured = [];
  class Host extends EventTarget {
    addEventListener(type, listener, options) {
      captured.push([type, typeof options === "object" && options.capture]);
      super.addEventListener(type, listener, options);
    }
  }
  const { watch } = recorder("?" + INSPECT_QUERY);
  watch({ host: new Host(), addon: addonDouble(), terminal: "term-1", pane: "conversation" });

  assert.deepEqual(captured, [["webglcontextlost", true], ["webglcontextrestored", true]]);
});

// Breaks if a release or a fallback goes unrecorded, or if the created total
// falls when a renderer goes and stops counting every context the page made.
test("releases and fallbacks are recorded beside a created total that only rises", () => {
  const { sent, watch } = recorder("?" + INSPECT_QUERY);
  const renderer = () => ({ host: new EventTarget(), addon: addonDouble(), terminal: "term-1", pane: "conversation" });

  watch(renderer())("released");
  watch(renderer())("fallback");
  watch(renderer());

  assert.deepEqual(sent, [
    ["created", "term-1", "conversation", 1, 1],
    ["released", "term-1", "conversation", 0, 1],
    ["created", "term-1", "conversation", 1, 2],
    ["fallback", "term-1", "conversation", 0, 2],
    ["created", "term-1", "conversation", 1, 3],
  ]);
});
