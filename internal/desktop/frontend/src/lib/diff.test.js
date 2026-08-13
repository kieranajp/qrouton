import assert from "node:assert/strict";
import test from "node:test";
import { parseDiff } from "./diff.js";

const positions = (text) => parseDiff(text).rows.map(({ kind, oldLine, newLine }) => [
  kind,
  oldLine,
  newLine,
]);

test("a complete hunk numbers context, deletion and addition on their own sides", () => {
  const parsed = parseDiff([
    "@@ -10,3 +20,3 @@ describe section",
    " first",
    "-old",
    "+new",
    " last",
  ].join("\n"));

  assert.deepEqual(positions(parsed.rows.map((row) => row.text).join("\n")), [
    ["hunk", null, null],
    ["context", 10, 20],
    ["del", 11, null],
    ["add", null, 21],
    ["context", 12, 22],
  ]);
  assert.equal(parsed.digits, 2);
});

test("omitted counts mean one and may number a line at zero", () => {
  assert.deepEqual(positions("@@ -0 +0 @@\n-old\n+new"), [
    ["hunk", null, null],
    ["del", 0, null],
    ["add", null, 0],
  ]);
});

test("zero-count ranges support pure additions and pure deletions", () => {
  assert.deepEqual(positions([
    "@@ -0,0 +5,2 @@",
    "+one",
    "+two",
    "separator",
    "@@ -8,2 +0,0 @@",
    "-one",
    "-two",
  ].join("\n")), [
    ["hunk", null, null],
    ["add", null, 5],
    ["add", null, 6],
    ["plain", null, null],
    ["hunk", null, null],
    ["del", 8, null],
    ["del", 9, null],
  ]);
});

test("later hunks and files reset both starts", () => {
  assert.deepEqual(positions([
    "diff --git a/one b/one",
    "@@ -2 +12 @@",
    " first",
    "diff --git a/two b/two",
    "@@ -40,1 +7,1 @@ second",
    " second",
  ].join("\n")), [
    ["file", null, null],
    ["hunk", null, null],
    ["context", 2, 12],
    ["file", null, null],
    ["hunk", null, null],
    ["context", 40, 7],
  ]);
});

test("no-newline markers consume neither side", () => {
  assert.deepEqual(positions([
    "@@ -7 +9 @@",
    "-old",
    "\\ No newline at end of file",
    "+new",
    "\\ No newline at end of file",
  ].join("\n")), [
    ["hunk", null, null],
    ["del", 7, null],
    ["marker", null, null],
    ["add", null, 9],
    ["marker", null, null],
  ]);
});

test("metadata, binary notices, separators and presentation blanks stay unnumbered", () => {
  const metadata = [
    "=== app ===",
    "diff --git a/logo.png b/logo.png",
    "index 123..456 100644",
    "--- a/logo.png",
    "+++ b/logo.png",
    "new file mode 100644",
    "deleted file mode 100644",
    "rename from old.png",
    "rename to new.png",
    "similarity index 100%",
    "old mode 100644",
    "new mode 100755",
    "Binary files a/logo.png and b/logo.png differ",
  ];
  const text = [
    ...metadata,
    "",
    "",
  ].join("\n");
  const parsed = parseDiff(text);

  assert.deepEqual(parsed.rows.map((row) => row.text), text.split("\n"));
  assert.deepEqual(positions(text), [
    ...metadata.map(() => ["file", null, null]),
    ["plain", null, null],
    ["plain", null, null],
  ]);
  assert.equal(parsed.digits, 1);
});

test("metadata-like changed content keeps its hunk grammar", () => {
  assert.deepEqual(positions("@@ -3 +4 @@\n--- content\n+++ content"), [
    ["hunk", null, null],
    ["del", 3, null],
    ["add", null, 4],
  ]);
});

test("malformed and combined headers never establish numbering state", () => {
  const parsed = parseDiff([
    "@@ -1,1 +1,1",
    " outside context",
    "-outside deletion",
    "+outside addition",
    "@@@ -2,1 -3,1 +4,1 @@@",
    " combined context",
    "-combined deletion",
    "+combined addition",
  ].join("\n"));

  assert.ok(parsed.rows.every((row) => row.oldLine === null && row.newLine === null));
  assert.deepEqual(parsed.rows.map((row) => row.kind), [
    "hunk",
    "plain",
    "del",
    "add",
    "hunk",
    "plain",
    "del",
    "add",
  ]);
});

test("a valid hunk recovers independently after malformed input", () => {
  assert.deepEqual(positions([
    "@@@ -2,1 -3,1 +4,1 @@@",
    " combined context",
    "@@ -70 +80 @@",
    " recovered",
  ].join("\n")), [
    ["hunk", null, null],
    ["plain", null, null],
    ["hunk", null, null],
    ["context", 70, 80],
  ]);
});

test("an under-consumed hunk at EOF clears every buffered coordinate", () => {
  assert.deepEqual(positions("@@ -3,2 +8,2 @@\n only one"), [
    ["hunk", null, null],
    ["context", null, null],
  ]);
});

test("a new header closes an under-consumed hunk and recovers independently", () => {
  assert.deepEqual(positions([
    "@@ -3,2 +8,2 @@",
    " only one",
    "@@ -50 +60 @@",
    " complete",
  ].join("\n")), [
    ["hunk", null, null],
    ["context", null, null],
    ["hunk", null, null],
    ["context", 50, 60],
  ]);
});

test("body rows beyond the declared counts invalidate the whole hunk", () => {
  assert.deepEqual(positions("@@ -3 +8 @@\n first\n extra"), [
    ["hunk", null, null],
    ["context", null, null],
    ["context", null, null],
  ]);
});

test("unsafe starts, counts and consumed coordinates fall back without throwing", () => {
  const headers = [
    "@@ -9007199254740992 +1 @@",
    "@@ -1,9007199254740992 +1 @@",
    "@@ -9007199254740991,2 +1 @@",
  ];
  for (const header of headers) {
    assert.doesNotThrow(() => parseDiff(`${header}\n-old\n+new`));
    assert.deepEqual(positions(`${header}\n-old\n+new`), [
      ["hunk", null, null],
      ["del", null, null],
      ["add", null, null],
    ]);
  }
});

test("large safe coordinates remain exact and set the shared digit width", () => {
  const parsed = parseDiff("@@ -9007199254740991 +0,0 @@\n-last");
  assert.deepEqual(positions(parsed.rows.map((row) => row.text).join("\n")), [
    ["hunk", null, null],
    ["del", 9007199254740991, null],
  ]);
  assert.equal(parsed.digits, 16);
});

test("only the anchored ASCII two-way header grammar starts a candidate", () => {
  const invalid = [
    "@@ -1 +2 @@tail",
    "@@ -1 +2 @@@",
    "@@ 1 +2 @@",
    "@@ -1 2 @@",
    "@@ --1 +2 @@",
    "@@ -1 +٢ @@",
    "\u001b[36m@@ -1 +2 @@\u001b[0m",
  ];
  for (const header of invalid) {
    const parsed = parseDiff(`${header}\n row`);
    assert.ok(parsed.rows.every((row) => row.oldLine === null && row.newLine === null), header);
  }
});
