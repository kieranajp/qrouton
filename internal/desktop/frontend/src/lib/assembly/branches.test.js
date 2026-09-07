import assert from "node:assert/strict";
import test from "node:test";
import { entry, requested, settled, unasked, wanted } from "./branches.js";

test("entry answers idle for an id nothing has asked about", () => {
  assert.deepEqual(entry(unasked(), "acme/api"), { state: "idle", branches: [], default: "" });
});

test("wanted is true for idle and failed, false for loading and ready", () => {
  const lists = {
    api: { state: "idle", branches: [], default: "" },
    docs: { state: "failed", branches: [], default: "", error: "boom" },
    web: { state: "loading", branches: [], default: "" },
    cli: { state: "ready", branches: ["main"], default: "main" },
  };
  assert.equal(wanted(lists, "api"), true);
  assert.equal(wanted(lists, "docs"), true);
  assert.equal(wanted(lists, "web"), false);
  assert.equal(wanted(lists, "cli"), false);
});

test("requested leaves a ready or loading entry untouched", () => {
  const ready = { state: "ready", branches: ["main"], default: "main" };
  const loading = { state: "loading", branches: [], default: "" };
  const lists = { api: ready, docs: loading };
  const result = requested(lists, "api");
  assert.equal(result, lists);
  assert.equal(result.api, ready);
  assert.equal(requested(lists, "docs").docs, loading);
});

test("requested moves an idle or failed entry to loading and clears its error", () => {
  const lists = { api: { state: "failed", branches: [], default: "", error: "boom" } };
  assert.deepEqual(requested(lists, "api").api, {
    state: "loading",
    branches: [],
    default: "",
    error: undefined,
  });
});

test("settled folds a successful answer into a ready entry", () => {
  const lists = requested(unasked(), "acme/api");
  const result = settled(lists, "acme/api", { branches: ["main", "dev"], default: "main" });
  assert.deepEqual(result["acme/api"], {
    state: "ready",
    branches: ["main", "dev"],
    default: "main",
    error: undefined,
  });
});

// Go answers a failure alongside the fallback branches it already chose, so
// the menu still has something to offer.
test("settled folds a failed answer into a failed entry, keeping Go's fallback branches", () => {
  const lists = requested(unasked(), "acme/api");
  const result = settled(lists, "acme/api", {
    branches: ["main"],
    default: "main",
    error: "network unreachable",
  });
  assert.deepEqual(result["acme/api"], {
    state: "failed",
    branches: ["main"],
    default: "main",
    error: "network unreachable",
  });
});

test("settled tolerates a missing or partial answer", () => {
  assert.deepEqual(settled(unasked(), "acme/api", undefined), {
    "acme/api": { state: "ready", branches: [], default: "", error: undefined },
  });
  assert.deepEqual(settled(unasked(), "acme/api", { branches: ["main"] }), {
    "acme/api": { state: "ready", branches: ["main"], default: "", error: undefined },
  });
});
