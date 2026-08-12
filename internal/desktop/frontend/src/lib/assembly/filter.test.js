import assert from "node:assert/strict";
import test from "node:test";
import { filter, repoID } from "./filter.js";

// Activity order, newest first, as Go hands it over.
const REPOS = [
  { org: "lifesum", name: "api" },
  { org: "lifesum", name: "billing" },
  { org: "vimeda", name: "billing-legacy" },
  { org: "kieranajp", name: "billboard" },
];

const ids = (rows) => rows.map((row) => row.id).join(" ");

test("an owner switched off drops its repositories", () => {
  const { rows } = filter({ repos: REPOS, owners: ["lifesum", "kieranajp"] });
  assert.equal(ids(rows), "lifesum/api lifesum/billing kieranajp/billboard");
});

test("no owner selected shows nothing rather than everything", () => {
  assert.deepEqual(filter({ repos: REPOS, owners: [] }).rows, []);
});

test("the search matches org and name together, whatever its case", () => {
  const owners = ["lifesum", "vimeda", "kieranajp"];
  assert.equal(ids(filter({ repos: REPOS, owners, query: "BILL" }).rows),
    "lifesum/billing vimeda/billing-legacy kieranajp/billboard");
  assert.equal(ids(filter({ repos: REPOS, owners, query: "vimeda/bil" }).rows),
    "vimeda/billing-legacy");
});

test("filtering preserves activity order and leaves the source list alone", () => {
  const { rows } = filter({ repos: REPOS, owners: ["kieranajp", "lifesum"], query: "bill" });
  assert.equal(ids(rows), "lifesum/billing kieranajp/billboard");
  assert.equal(repoID(REPOS[0]), "lifesum/api");
  assert.equal(REPOS.length, 4);
});

test("the count reports the whole cache and what is shown of it", () => {
  const owners = ["lifesum", "vimeda", "kieranajp"];
  const all = filter({ repos: REPOS, owners });
  assert.deepEqual([all.shown, all.total], [4, 4]);

  const narrowed = filter({ repos: REPOS, owners, query: "bill" });
  assert.deepEqual([narrowed.shown, narrowed.total], [3, 4]);
});

test("the cap changes what is shown without changing the total", () => {
  const capped = filter({ repos: REPOS, owners: ["lifesum", "vimeda", "kieranajp"], cap: 2 });
  assert.equal(ids(capped.rows), "lifesum/api lifesum/billing");
  assert.deepEqual([capped.shown, capped.total], [2, 4]);
});
