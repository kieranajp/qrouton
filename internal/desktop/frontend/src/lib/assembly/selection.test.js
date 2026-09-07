import assert from "node:assert/strict";
import test from "node:test";
import {
  counts,
  isLocked,
  isUpgrading,
  ordered,
  preselect,
  reconcile,
  roleOf,
  roleOffers,
  rowMeta,
  seed,
  setRole,
  summary,
  upgrading,
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

// An editing worktree can carry commits and uncommitted work, so the picker
// neither demotes it nor drops it — and composing it again would clone it twice.
test("a held editing row answers to nothing and never reaches the ordered array", () => {
  let selection = seed([{ id: "acme/api", role: "editing" }]);
  assert.equal(roleOf(selection, "acme/api"), "editing");
  assert.ok(isLocked(selection, "acme/api"));
  assert.deepEqual(roleOffers(selection, "acme/api"), []);
  assert.equal(picked(selection), "");

  for (const role of ["reference", "off"]) {
    selection = setRole(selection, "acme/api", role);
    assert.equal(roleOf(selection, "acme/api"), "editing");
    assert.deepEqual(upgrading(selection), []);
  }
  assert.equal(picked(selection), "");

  selection = setRole(selection, "other/web", "editing");
  assert.equal(picked(selection), "other/web");
});

// The point of the toggle beside a held reference row: it reaches Go as an
// upgrade, not as a row to compose, because the two are different operations.
test("a held reference row can be taken up for editing, and put back", () => {
  let selection = seed([{ id: "acme/docs", role: "reference" }]);
  assert.deepEqual(roleOffers(selection, "acme/docs"), ["reference", "editing"]);

  selection = setRole(selection, "acme/docs", "editing");
  assert.equal(roleOf(selection, "acme/docs"), "editing");
  assert.ok(isUpgrading(selection, "acme/docs"));
  assert.deepEqual(upgrading(selection), ["acme/docs"]);
  assert.equal(picked(selection), "");

  selection = setRole(selection, "acme/docs", "reference");
  assert.equal(roleOf(selection, "acme/docs"), "reference");
  assert.deepEqual(upgrading(selection), []);
});

// Turning a held row off would mean removing a checkout, which the picker does
// not do — so the toggle does not offer it.
test("a held reference row cannot be turned off", () => {
  let selection = setRole(seed([{ id: "acme/docs", role: "reference" }]), "acme/docs", "editing");
  selection = setRole(selection, "acme/docs", "off");
  assert.equal(roleOf(selection, "acme/docs"), "editing");
  assert.deepEqual(upgrading(selection), ["acme/docs"]);
});

test("a held row says what the session does with it, and says it once taken up", () => {
  const editing = seed([{ id: "acme/api", role: "editing" }]);
  assert.equal(rowMeta(editing, "acme/api", "pushed 2h ago"), "pushed 2h ago · in session");
  assert.equal(rowMeta(editing, "acme/api", ""), "in session");
  assert.equal(rowMeta(editing, "other/web", "pushed 2h ago"), "pushed 2h ago");

  const reading = seed([{ id: "acme/docs", role: "reference" }]);
  assert.equal(rowMeta(reading, "acme/docs", ""), "in session, read-only");
  assert.equal(
    rowMeta(setRole(reading, "acme/docs", "editing"), "acme/docs", ""),
    "in session, taking it up to edit",
  );
});

// A free row's toggle answers to everything, which is what the wizard draws.
test("a row the session does not hold answers to every role", () => {
  assert.deepEqual(roleOffers(seed(), "acme/api"), ["off", "editing", "reference"]);
});

test("a locked row survives a refresh that no longer lists it", () => {
  const selection = reconcile(seed([{ id: "acme/api", role: "editing" }]), ["other/web"]);
  assert.equal(roleOf(selection, "acme/api"), "editing");
});

// The session holds it whatever GitHub now reports, so it is still upgradable.
test("taking a held row up survives a refresh that no longer lists it", () => {
  const taken = setRole(seed([{ id: "acme/docs", role: "reference" }]), "acme/docs", "editing");
  const selection = reconcile(taken, ["other/web"]);
  assert.deepEqual(upgrading(selection), ["acme/docs"]);
  assert.equal(roleOf(selection, "acme/docs"), "editing");
});

test("the role counts follow the rows, locked ones included", () => {
  let selection = seed([{ id: "acme/api", role: "editing" }]);
  selection = setRole(selection, "acme/docs", "editing");
  selection = setRole(selection, "other/web", "reference");
  assert.deepEqual(counts(selection), { editing: 2, reference: 1 });

  selection = setRole(selection, "acme/docs", "off");
  assert.deepEqual(counts(selection), { editing: 1, reference: 1 });
});

test("a held row taken up counts against editing, not reference", () => {
  const reading = seed([{ id: "acme/docs", role: "reference" }]);
  assert.deepEqual(counts(reading), { editing: 0, reference: 1 });
  assert.deepEqual(counts(setRole(reading, "acme/docs", "editing")), { editing: 1, reference: 0 });
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

// A held row scrolls out of sight behind a search; the chip is where an answer
// the user has given stays visible until they confirm it.
test("a row being taken up leads the chips, naming the branch it joins", () => {
  const repos = [{ org: "acme", name: "docs", default_branch: "main" }];
  let selection = seed([{ id: "acme/docs", role: "reference" }]);
  assert.deepEqual(summary(selection, repos, "feat/extract-billing"), []);

  selection = setRole(selection, "acme/docs", "editing");
  selection = setRole(selection, "other/web", "reference");
  assert.deepEqual(summary(selection, repos, "feat/extract-billing"), [
    { id: "acme/docs", role: "editing", glyph: "●", meta: "→ feat/extract-billing" },
    { id: "other/web", role: "reference", glyph: "◐", meta: "→ read-only" },
  ]);

  selection = setRole(selection, "acme/docs", "reference");
  assert.deepEqual(ordered(selection).map((row) => row.id), ["other/web"]);
  assert.equal(summary(selection, repos, "feat/extract-billing").length, 1);
});

// An agent's request ticks rows the same way a person's click would.
test("preselect ticks an unheld row at the asked role", () => {
  const selection = preselect(seed(), [{ id: "acme/api", role: "editing" }]);
  assert.equal(roleOf(selection, "acme/api"), "editing");
  assert.equal(picked(selection), "acme/api");
});

test("preselect marks a held reference row as upgrading", () => {
  const selection = preselect(seed([{ id: "acme/docs", role: "reference" }]), [
    { id: "acme/docs", role: "editing" },
  ]);
  assert.ok(isUpgrading(selection, "acme/docs"));
  assert.equal(roleOf(selection, "acme/docs"), "editing");
});

test("preselect is a no-op for an empty request", () => {
  const selection = seed([{ id: "acme/api", role: "editing" }]);
  assert.deepEqual(preselect(selection, []), selection);
});

// An agent's request is ticked before the list has been refreshed, so the id it
// named is the only spelling available. A refresh that turns it up under
// GitHub's own casing has to keep the tick, not drop it as a row nothing lists.
test("a picked row is respelled by the refreshed list that carries it", () => {
  const selection = reconcile(preselect(seed(), [{ id: "acme/api", role: "editing" }]), [
    "Acme/API",
  ]);
  assert.equal(roleOf(selection, "Acme/API"), "editing");
  assert.equal(roleOf(selection, "acme/api"), "off");
  assert.deepEqual(ordered(selection), [{ id: "Acme/API", role: "editing" }]);
});

test("a held row keeps the spelling the session holds it under", () => {
  const held = seed([{ id: "acme/docs", role: "reference" }]);
  const selection = reconcile(setRole(held, "acme/docs", "editing"), ["Acme/DOCS"]);
  assert.deepEqual(upgrading(selection), ["acme/docs"]);
  assert.equal(roleOf(selection, "acme/docs"), "editing");
});
