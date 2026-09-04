import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";

const theme = readFileSync(fileURLToPath(new URL("./slide-theme.css", import.meta.url)), "utf8");

// The whole vocabulary an author may write: four layouts, five components, and
// the modifiers that compose with them. A tenth name is a theme band nobody
// asked for, so it fails here rather than in review.
const VOCABULARY = new Set([
  "title",
  "statement",
  "alt",
  "cols",
  "wide-left",
  "wide-right",
  "shot",
  "cards",
  "accent",
  "callout",
  "note",
  "good",
  "warn",
]);

test("the theme styles only the closed vocabulary", () => {
  const used = new Set([...theme.matchAll(/\.([a-z][a-z0-9-]*)/gi)].map(([, name]) => name));
  assert.deepEqual([...used].filter((name) => !VOCABULARY.has(name)), []);
});

test("every colour and face is a token the app serves", () => {
  assert.doesNotMatch(theme, /#[0-9a-f]{3,8}\b/i);
  assert.doesNotMatch(theme, /@import/);
  assert.doesNotMatch(theme, /url\(/);
  for (const [, value] of theme.matchAll(/font-family:\s*([^;]+);/g)) {
    assert.match(value.trim(), /^var\(--font-[a-z]+\)$/);
  }
});

test("the theme declares itself Marp's and stays one screenful of bands", () => {
  assert.match(theme, /^\/\* @theme qrouton \*\//);
  assert.ok(theme.split("\n").length < 300, "the theme has grown a band nobody asked for");
});
