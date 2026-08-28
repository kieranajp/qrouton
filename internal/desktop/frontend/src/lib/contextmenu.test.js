import assert from "node:assert/strict";
import test from "node:test";
import { itemsFor } from "./contextmenu.js";

const labels = (items) => items.filter((item) => item !== "-").map((item) => item.label);
const acts = (items) => Object.fromEntries(items.filter((i) => i !== "-").map((i) => [i.act, i]));

test("a click on inert chrome opens no menu at all", () => {
  assert.deepEqual(itemsFor(), []);
  assert.deepEqual(itemsFor({ kind: "chrome" }), []);
});

test("text offers a copy only once something is selected", () => {
  assert.deepEqual(itemsFor({ kind: "text" }), []);
  assert.deepEqual(labels(itemsFor({ kind: "text", selection: "picked" })), ["Copy"]);
});

// The menu keeps its shape between clicks: what cannot act is drawn dimmed
// rather than dropped, so items do not move under the pointer.
test("a terminal always offers the same items, with copy dimmed until there is a selection", () => {
  const empty = itemsFor({ kind: "terminal" });
  const selected = itemsFor({ kind: "terminal", selection: "output" });
  assert.deepEqual(labels(empty), labels(selected));
  assert.equal(acts(empty).copy.disabled, true);
  assert.equal(acts(selected).copy.disabled, false);
  assert.equal(acts(empty).paste.disabled, undefined);
});

test("a writable field can cut and paste, a read-only one neither", () => {
  const writable = labels(itemsFor({ kind: "field", selection: "typed" }));
  assert.deepEqual(writable, ["Cut", "Copy", "Paste", "Select All"]);
  const locked = itemsFor({ kind: "field", selection: "typed", writable: false });
  assert.deepEqual(labels(locked), ["Copy", "Select All"]);
});

test("cutting is dimmed with nothing selected", () => {
  assert.equal(acts(itemsFor({ kind: "field" })).cut.disabled, true);
});

test("an external link is opened or copied, whatever the selection", () => {
  const open = ["Open Link", "Copy Link"];
  assert.deepEqual(labels(itemsFor({ kind: "link", linkKind: "external" })), open);
  assert.deepEqual(
    labels(itemsFor({ kind: "link", linkKind: "external", selection: "text" })),
    open,
  );
});

test("a document link is opened in the pane it names", () => {
  assert.deepEqual(labels(itemsFor({ kind: "link", linkKind: "document" })), [
    "Open Link",
    "Copy Link",
  ]);
});

// An unfollowable link would otherwise offer an item that silently does nothing.
test("an unclassified link only offers a copy", () => {
  assert.deepEqual(labels(itemsFor({ kind: "link", linkKind: "none" })), ["Copy Link"]);
});

// The rules hand out one object per call; a shared item would carry a stale
// disabled flag into the next menu.
test("each menu is built fresh", () => {
  const first = acts(itemsFor({ kind: "terminal" })).copy;
  const second = acts(itemsFor({ kind: "terminal", selection: "output" })).copy;
  assert.notEqual(first, second);
  assert.equal(first.disabled, true);
});
