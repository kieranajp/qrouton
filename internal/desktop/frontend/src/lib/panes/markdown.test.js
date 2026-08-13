import assert from "node:assert/strict";
import { test } from "node:test";
import { documentPath, linkKind, marks, render } from "./markdown.js";

/** @param {string} body */
const numbered = (body) => [...body.matchAll(/<(\w+)[^>]*data-line="(\d+)"/g)].map((m) => [m[1], Number(m[2])]);

const PLAN = [
  "---",
  "date: 2026-08-11",
  "tags: [plan]",
  "---",
  "",
  "# Phase 3 — move the repository",
  "",
  "> Stop rule: if `go test ./...` is red, do not start phase 4.",
  "",
  "- [x] the port is defined",
  "- [ ] the tests pass",
  "",
  "```go",
  "// port/invoices.go",
  "type InvoiceRepository interface {",
  "\tSave(ctx context.Context) error",
  "}",
  "```",
].join("\n");

test("front matter never reaches the pane", () => {
  const { body } = render(PLAN);
  assert.ok(!body.includes("2026-08-11"));
  assert.ok(!body.includes("tags"));
});

test("the opening heading becomes the title and leaves the body", () => {
  const { title, body } = render(PLAN);
  assert.equal(title, "Phase 3 — move the repository");
  assert.ok(!body.includes("<h1>"));
});

test("a document with no opening heading keeps its title empty", () => {
  const { title, body } = render("Just a note.\n\n# Later heading\n");
  assert.equal(title, "");
  assert.ok(body.includes(">Later heading</h1>"));
});

test("task list state survives as markup the pane can draw", () => {
  const { body } = render(PLAN);
  assert.ok(body.includes('class="contains-task-list"'));
  assert.ok(body.includes('<input type="checkbox" checked disabled>'));
  assert.ok(body.includes('<input type="checkbox" disabled>'));
});

test("highlighting survives the sanitiser as classes, not inline styles", () => {
  const { body } = render(PLAN);
  assert.ok(body.includes("sh__token--keyword"));
  assert.ok(body.includes("sh__token--comment"));
  assert.ok(!body.includes("style="));
});

test("a fence in no language, and in one sugar-high does not know, still render", () => {
  for (const fence of ["```\nplain text\n```", "```brainfuck\n++++.\n```"]) {
    const { body } = render(fence);
    assert.ok(body.includes("<pre"), fence);
  }
});

test("script and event handlers do not survive the sanitiser", () => {
  const { body } = render("<script>alert(1)</script>\n\n<img src=x onerror=alert(2)>\n");
  assert.ok(!body.includes("<script"));
  assert.ok(!body.includes("onerror"));
  assert.ok(!body.includes("alert"));
});

test("every block carries the source line it opens on, front matter included in the count", () => {
  assert.deepEqual(numbered(render(PLAN).body), [
    ["blockquote", 8],
    ["li", 10],
    ["li", 11],
    ["pre", 13],
  ]);
});

test("a block spanning several lines reports the line it ends on too", () => {
  const { body } = render("first\nstill first\n\nsecond\n");
  assert.ok(body.includes('<p data-line="1" data-line-end="2">'));
  assert.ok(body.includes('<p data-line="4" data-line-end="4">'));
});

test("a list defers its number to its items, at every depth", () => {
  const { body } = render("- one\n- two\n  - nested\n");
  assert.deepEqual(numbered(body), [
    ["li", 1],
    ["li", 2],
    ["li", 3],
  ]);
  assert.ok(!body.includes("<ul data-line"));
});

test("a fence keeps its line despite the highlighter rebuilding it", () => {
  const { body } = render("intro\n\n```go\nx := 1\n```\n\n```\nplain\n```\n");
  assert.deepEqual(numbered(body), [
    ["p", 1],
    ["pre", 3],
    ["pre", 7],
  ]);
});

test("marks covers the blocks a span reaches and scrolls to the first", () => {
  const blocks = [
    { line: 3, end: 3 },
    { line: 5, end: 8 },
    { line: 12, end: 12 },
  ];
  assert.deepEqual(marks(blocks, { line: 5, to: 0 }), { marked: [1], at: 1 });
  assert.deepEqual(marks(blocks, { line: 7, to: 12 }), { marked: [1, 2], at: 1 });
  assert.deepEqual(marks(blocks, { line: 1, to: 20 }), { marked: [0, 1, 2], at: 0 });
});

test("a span landing between blocks marks nothing and scrolls to the next one", () => {
  const blocks = [
    { line: 3, end: 3 },
    { line: 9, end: 9 },
  ];
  assert.deepEqual(marks(blocks, { line: 5, to: 6 }), { marked: [], at: 1 });
});

test("a span past the last block, or naming no line, reaches nothing", () => {
  const blocks = [{ line: 3, end: 3 }];
  assert.deepEqual(marks(blocks, { line: 40, to: 0 }), { marked: [], at: -1 });
  assert.deepEqual(marks(blocks, { line: 0, to: 0 }), { marked: [], at: -1 });
});

test("linkKind separates a document from a URL from everything else", () => {
  assert.equal(linkKind("../research/R7-2026-08-07-editor-surfaces.md"), "document");
  assert.equal(linkKind("plans/P007.markdown#phase-2"), "document");
  assert.equal(linkKind("https://example.com"), "external");
  assert.equal(linkKind("#a-heading"), "none");
  assert.equal(linkKind("src/main.go"), "none");
  assert.equal(linkKind("file:///etc/passwd"), "none");
  assert.equal(linkKind("javascript:alert(1)"), "none");
  assert.equal(linkKind(null), "none");
});

test("a document link resolves against the document holding it", () => {
  const from = "thoughts/shared/plans/P007.md";
  assert.equal(documentPath("../research/R7.md", from), "thoughts/shared/research/R7.md");
  assert.equal(documentPath("./P006.md", from), "thoughts/shared/plans/P006.md");
  assert.equal(documentPath("P006.md#phase-1", from), "thoughts/shared/plans/P006.md");
  assert.equal(documentPath("/AGENTS.md", from), "AGENTS.md");
});
