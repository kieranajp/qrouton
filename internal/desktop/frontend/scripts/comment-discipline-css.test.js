import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { checkTree } from "./comment-discipline-css.js";

const policy = {
  schemaVersion: 1,
  maxCommentRun: 2,
  narrationPhrases: ["the problem was"],
  pathExtensions: ["js", "css"],
};

test("standalone CSS comments are checked with original locations", () => {
  const diagnostics = checkTreeWithFiles({ "styles.css": "/* one\n * two\n */\n.button {}" }, policy);
  assert.equal(diagnostics.length, 1);
  assert.match(diagnostics[0], /^styles\.css:1:1: comment-discipline\/max-comment-run:/);
});

test("Svelte style CSS is checked at the original file location", () => {
  const diagnostics = checkTreeWithFiles({
    "Card.svelte": "<script>let value = 1;</script>\n<style>\n  /* The problem was the stale ref. */\n  .card {}\n</style>",
  }, policy);
  assert.equal(diagnostics.length, 1);
  assert.match(diagnostics[0], /^Card\.svelte:3:3: comment-discipline\/no-narration:/);
});

test("directives terminate runs and URLs are removed span by span", () => {
  const diagnostics = checkTreeWithFiles({
    "styles.css": "/* one */\n/* eslint-disable */\n/* two */\n/* See https://example.com/a/b.js and src/foo.js. */",
  }, policy);
  assert.equal(diagnostics.length, 1);
  assert.match(diagnostics[0], /styles\.css:4:1: comment-discipline\/no-path-pointer/);
});

test("CSS traversal covers authored dot directories and skips node_modules", () => {
  const source = "/* The problem was the stale ref. */";
  const diagnostics = checkTreeWithFiles({ ".fixtures/authored.css": source, "node_modules/local.css": source }, policy);
  assert.equal(diagnostics.length, 1);
  assert.match(diagnostics[0], /^\.fixtures\/authored\.css:1:1:/);
});

test("CSS parse failures carry a stable location and rule", () => {
  assert.throws(
    () => checkTreeWithFiles({ "broken.css": "/* unclosed" }, policy),
    /broken\.css:1:1: comment-discipline\/parse-error:/,
  );
});

function checkTreeWithFiles(files, policy) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "comment-discipline-"));
  for (const [name, contents] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(root, name)), { recursive: true });
    fs.writeFileSync(path.join(root, name), contents);
  }
  return checkTree(root, policy);
}
