import assert from "node:assert/strict";
import test from "node:test";
import { slideSpans } from "./slide-source.js";

test("frontmatter belongs to no slide", () => {
  const deck = "---\nmarp: true\n---\n\n# One\n\n---\n\n# Two\n";
  assert.deepEqual(slideSpans(deck), [
    { line: 4, lineEnd: 6 },
    { line: 8, lineEnd: 9 },
  ]);
});

test("a deck without frontmatter starts at its first line", () => {
  assert.deepEqual(slideSpans("# One\n\n---\n\n# Two\n"), [
    { line: 1, lineEnd: 2 },
    { line: 4, lineEnd: 5 },
  ]);
});

test("a separator inside a code fence opens nothing", () => {
  const deck = "# One\n\n```md\n\n---\n\n```\n\n---\n\n# Two\n";
  assert.deepEqual(slideSpans(deck), [
    { line: 1, lineEnd: 8 },
    { line: 10, lineEnd: 11 },
  ]);
});

test("a trailing separator adds no empty slide", () => {
  assert.deepEqual(slideSpans("# One\n\n---\n\n"), [{ line: 1, lineEnd: 2 }]);
});

test("a setext underline is not a slide break", () => {
  assert.deepEqual(slideSpans("Heading\n---\n\nbody\n"), [{ line: 1, lineEnd: 4 }]);
});

test("a single-slide deck is one span", () => {
  assert.deepEqual(slideSpans("---\nmarp: true\n---\n\n# Only\n"), [{ line: 4, lineEnd: 5 }]);
});

test("an unclosed frontmatter block leaves nothing to slice", () => {
  assert.deepEqual(slideSpans("---\nmarp: true\n\n# One\n"), []);
});

test("an empty deck has no slides", () => {
  assert.deepEqual(slideSpans(""), []);
  assert.deepEqual(slideSpans(undefined), []);
});
