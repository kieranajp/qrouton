import assert from "node:assert/strict";
import test from "node:test";
import {
  counts,
  isLocked,
  ordered,
  reconcile,
  roleOf,
  rowMeta,
  seed,
  setRole,
  summary,
} from "./selection.js";

const picked = (selection) => ordered(selection).map((row) => row.id).join(" ");

test("pick order is the order picked, not the order the list draws", () => {
  let selection = seed();
  selection = setRole(selection, "other/web", "editing");
  selection = setRole(selection, "acme/api", "editing");
  selection = setRole(selection, "acme/docs", "reference");
  assert.equal(picked(selection), "other/web acme/api acme/docs");
});

test("demoting a repository to reference keeps its rank", () => {
  let selection = seed();
  selection = setRole(selection, "acme/api", "editing");
  selection = setRole(selection, "other/web", "editing");
  selection = setRole(selection, "acme/api", "reference");
  assert.equal(picked(selection), "acme/api other/web");
  assert.equal(roleOf(selection, "acme/api"), "reference");
});

test("turning a repository off and back on picks it last", () => {
  let selection = seed();
  for (const id of ["acme/api", "acme/docs", "other/web"]) {
    selection = setRole(selection, id, "editing");
  }
  selection = setRole(selection, "acme/api", "off");
  assert.equal(picked(selection), "acme/docs other/web");
  assert.equal(roleOf(selection, "acme/api"), "off");

  selection = setRole(selection, "acme/api", "editing");
  assert.equal(picked(selection), "acme/docs other/web acme/api");
});

test("a repository a refresh dropped leaves the rest of the order intact", () => {
  let selection = seed();
  for (const id of ["acme/api", "acme/docs", "other/web"]) {
    selection = setRole(selection, id, "editing");
  }
  selection = reconcile(selection, ["acme/api", "other/web"]);
  assert.equal(picked(selection), "acme/api other/web");
  assert.equal(roleOf(selection, "acme/docs"), "off");
});

// Composing a repository the session already holds clones it a second time.
test("a locked row reports its role and never reaches the ordered array", () => {
  let selection = seed([{ id: "acme/api", role: "editing" }]);
  assert.equal(roleOf(selection, "acme/api"), "editing");
  assert.ok(isLocked(selection, "acme/api"));
  assert.equal(picked(selection), "");

  selection = setRole(selection, "acme/api", "reference");
  assert.equal(roleOf(selection, "acme/api"), "editing");
  assert.equal(picked(selection), "");

  selection = setRole(selection, "other/web", "editing");
  assert.equal(picked(selection), "other/web");
});

test("a held row says so beside its push time, and on its own without one", () => {
  assert.equal(rowMeta("pushed 2h ago", true), "pushed 2h ago · in session");
  assert.equal(rowMeta("", true), "in session");
  assert.equal(rowMeta("pushed 2h ago", false), "pushed 2h ago");
});

test("a locked row survives a refresh that no longer lists it", () => {
  const selection = reconcile(seed([{ id: "acme/api", role: "editing" }]), ["other/web"]);
  assert.equal(roleOf(selection, "acme/api"), "editing");
});

test("the role counts follow the rows, locked ones included", () => {
  let selection = seed([{ id: "acme/api", role: "editing" }]);
  selection = setRole(selection, "acme/docs", "editing");
  selection = setRole(selection, "other/web", "reference");
  assert.deepEqual(counts(selection), { editing: 2, reference: 1 });

  selection = setRole(selection, "acme/docs", "off");
  assert.deepEqual(counts(selection), { editing: 1, reference: 1 });
});

test("the summary names the branch editing repos join and the branch reference repos are read at", () => {
  const repos = [
    { org: "acme", name: "api", default_branch: "main" },
    { org: "other", name: "web", default_branch: "trunk" },
  ];
  let selection = seed();
  selection = setRole(selection, "acme/api", "editing");
  selection = setRole(selection, "other/web", "reference");
  assert.deepEqual(summary(selection, repos, "feat/extract-billing"), [
    { id: "acme/api", role: "editing", glyph: "●", meta: "→ feat/extract-billing" },
    { id: "other/web", role: "reference", glyph: "◐", meta: "→ trunk, read-only" },
  ]);
});

// A session with no repositories yet has no branch until its first ones land.
test("an editing chip names no branch while there is none", () => {
  const selection = setRole(seed(), "acme/api", "editing");
  assert.deepEqual(summary(selection, [], ""), [
    { id: "acme/api", role: "editing", glyph: "●", meta: "" },
  ]);
});
