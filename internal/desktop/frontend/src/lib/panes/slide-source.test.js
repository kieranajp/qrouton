import assert from "node:assert/strict";
import test from "node:test";
import { deckAssets, slideSpans } from "./slide-source.js";

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

test("relative media is addressed at the deck's asset route", () => {
  const deck = [
    '<img src="./shot.png">',
    '<video src="media/clip.mp4"></video>',
    '<source src="../shared/clip.webm">',
    "![alt](./shot.png)",
  ].join("\n");
  assert.equal(
    deckAssets(deck, "abc123"),
    [
      '<img src="/deck/abc123/shot.png">',
      '<video src="/deck/abc123/media/clip.mp4"></video>',
      '<source src="/deck/abc123/../shared/clip.webm">',
      "![alt](/deck/abc123/shot.png)",
    ].join("\n"),
  );
});

test("an absolute URL the author wrote is left alone", () => {
  const deck = '<img src="https://example.com/shot.png">\n![](data:image/png;base64,AAA)\n<img src="/already/rooted.png">';
  assert.equal(deckAssets(deck, "abc123"), deck);
});

test("a deck with no token keeps its own paths", () => {
  assert.equal(deckAssets('<img src="./shot.png">', ""), '<img src="./shot.png">');
  assert.equal(deckAssets('<img src="./shot.png">', undefined), '<img src="./shot.png">');
});

test("prose that merely mentions the route is not a rewrite target", () => {
  const deck = "The route is /deck/{token}/shot.png, and `./shot.png` is the source form.";
  assert.equal(deckAssets(deck, "abc123"), deck);
});
