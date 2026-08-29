import assert from "node:assert/strict";
import { test } from "node:test";
import { parseResearch } from "./research.js";

const doc = (...lines) => lines.join("\n");

const RESEARCH = doc(
  "---", // 1
  "kind: research", // 2
  "title: What we found", // 3
  "---", // 4
  "", // 5
  "# The question set", // 6
  "", // 7
  "Prose before anything opens.", // 8
  "", // 9
  "## Summary", // 10
  "", // 11
  "What it all came to.", // 12
  "", // 13
  "## How does the loader stamp a skill?", // 14
  "", // 15
  "It walks the folder.", // 16
  "", // 17
  "## Where does the kind come from?", // 18
  "", // 19
  "The path segment.", // 20
  "", // 21
  "## Open Questions", // 22
  "", // 23
  "Nothing outstanding.", // 24
  "",
);

test("the opening Summary is pinned and left out of the items", () => {
  const { title, summary, items } = parseResearch(RESEARCH);
  assert.equal(title, "The question set");
  assert.deepEqual([summary.index, summary.name], [0, "Summary"]);
  assert.deepEqual(
    items.map((item) => [item.index, item.name]),
    [
      [1, "How does the loader stamp a skill?"],
      [2, "Where does the kind come from?"],
      [3, "Open Questions"],
    ],
  );
});

test("item spans run to the line before the next opens", () => {
  const { preamble, summary, items } = parseResearch(RESEARCH);
  assert.deepEqual(preamble, { from: 5, to: 9 });
  assert.deepEqual([summary.from, summary.to], [10, 13]);
  assert.deepEqual(
    items.map((item) => [item.from, item.to]),
    [
      [14, 17],
      [18, 21],
      [22, 25],
    ],
  );
});

test("the heading text is matched case-insensitively and exactly", () => {
  const { summary } = parseResearch(doc("# Notes", "", "## summary", "", "Lower case.", ""));
  assert.equal(summary.name, "summary");
  assert.equal(parseResearch(doc("## Summary of findings", "", "Prose.", "")).summary, null);
});

test("a Summary that is not first is an item and nothing is pinned", () => {
  const { summary, items } = parseResearch(
    doc("# Notes", "", "## Background", "", "Prose.", "", "## Summary", "", "What it came to.", ""),
  );
  assert.equal(summary, null);
  assert.deepEqual(
    items.map((item) => item.name),
    ["Background", "Summary"],
  );
});

test("a document with no Summary pins nothing and keeps every section", () => {
  const { summary, items } = parseResearch(
    doc("# Notes", "", "## First question?", "", "Prose.", "", "## Second question?", "", "More.", ""),
  );
  assert.equal(summary, null);
  assert.deepEqual(
    items.map((item) => item.index),
    [0, 1],
  );
});

test("a heading quoted inside a fence opens nothing", () => {
  const { summary, items } = parseResearch(
    doc("# Notes", "", "```md", "## Summary", "", "## A question?", "```", "", "Prose.", ""),
  );
  assert.equal(summary, null);
  assert.deepEqual(items, []);
});

test("frontmatter naming a section opens nothing", () => {
  const { preamble, items } = parseResearch(
    doc("---", "title: Summary of the leftovers", "---", "", "# Notes", "", "Prose.", ""),
  );
  assert.deepEqual(items, []);
  assert.equal(preamble.from, 4);
});

test("a document with no second-level heading yields no items", () => {
  const { title, summary, items, preamble } = parseResearch(
    doc("# Just notes", "", "Nothing opens here.", ""),
  );
  assert.equal(title, "Just notes");
  assert.equal(summary, null);
  assert.deepEqual(items, []);
  assert.deepEqual(preamble, { from: 1, to: 4 });
});
