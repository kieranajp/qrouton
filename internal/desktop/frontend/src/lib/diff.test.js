import assert from "node:assert/strict";
import test from "node:test";
import { parseDiff, parsePatch } from "./diff.js";

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

const ordinaryFile = (path, oldLine, newLine) => [
  `diff --git a/${path} b/${path}`,
  "index 1111111..2222222 100644",
  `--- a/${path}`,
  `+++ b/${path}`,
  `@@ -${oldLine} +${newLine} @@`,
  "-old",
  "+new",
].join("\n");

function assertLossless(parsed) {
  let cursor = 0;
  for (const region of parsed.regions) {
    assert.equal(region.from, cursor);
    assert.ok(region.to > region.from);
    cursor = region.to;
  }
  assert.equal(cursor, parsed.raw.length);
  assert.equal(parsed.regions.map((region) => parsed.raw.slice(region.from, region.to)).join(""), parsed.raw);
}

test("ordinary files retain verbatim slices, paths, statuses and validated totals", () => {
  const raw = [
    ordinaryFile("one.txt", 1, 2),
    "diff --git a/two.txt b/two.txt",
    "new file mode 100644",
    "index 0000000..3333333",
    "--- /dev/null",
    "+++ b/two.txt",
    "@@ -0,0 +1,2 @@",
    "+first",
    "+second",
    "",
  ].join("\n");
  const parsed = parsePatch(raw);

  assert.equal(parsed.available, true);
  assert.equal(parsed.confidence, "full");
  assert.deepEqual(parsed.totals, { files: 2, additions: 3, deletions: 1, available: true });
  assert.deepEqual(parsed.files.map(({ path, status, additions, deletions }) => (
    { path, status, additions, deletions }
  )), [
    { path: "one.txt", status: "modified", additions: 1, deletions: 1 },
    { path: "two.txt", status: "added", additions: 2, deletions: 0 },
  ]);
  for (const file of parsed.files) {
    assert.equal(file.from, file.contentFrom);
    assert.equal(file.to, file.contentTo);
    assert.match(raw.slice(file.from, file.to), /^diff --git /);
  }
  assertLossless(parsed);
});

test("repository framing preserves blank separators, unchanged repositories and duplicate paths", () => {
  const first = ordinaryFile("same.txt", 1, 1);
  const third = ordinaryFile("same.txt", 7, 9);
  const raw = `\n=== src/one/ ===\n${first}\n\n=== src/two/ ===\n\n=== src/three/ ===\n${third}\n`;
  const parsed = parsePatch(raw);

  assert.deepEqual(parsed.repositories.map(({ name, filesChanged }) => ({ name, filesChanged })), [
    { name: "one", filesChanged: 1 },
    { name: "two", filesChanged: 0 },
    { name: "three", filesChanged: 1 },
  ]);
  assert.equal(parsed.files[0].path, parsed.files[1].path);
  assert.notEqual(parsed.files[0].id, parsed.files[1].id);
  assert.notEqual(parsed.files[0].repositoryId, parsed.files[1].repositoryId);
  assert.deepEqual(parsed.regions.filter((region) => region.kind === "repository")
    .map((region) => raw.slice(region.from, region.to)), [
    "\n=== src/one/ ===\n",
    "\n=== src/two/ ===\n",
    "\n=== src/three/ ===\n",
  ]);
  assert.equal(parsed.confidence, "full");
  assertLossless(parsed);
});

test("the exact no-change sentinel is explicit zero-change structure", () => {
  for (const raw of ["No changes in app.", "No changes in all session repos."]) {
    const parsed = parsePatch(raw);
    assert.equal(parsed.noChange.scope, raw.slice("No changes in ".length, -1));
    assert.equal(parsed.available, true);
    assert.equal(parsed.confidence, "full");
    assert.deepEqual(parsed.totals, { files: 0, additions: 0, deletions: 0, available: true });
    assert.deepEqual(parsed.regions.map((region) => region.kind), ["repository"]);
    assertLossless(parsed);
  }
  assert.equal(parsePatch("No changes in app.\n").noChange, null);
});

test("standard metadata derives rename, copy, binary and mode-only statuses", () => {
  const raw = [
    "diff --git a/old.txt b/new.txt",
    "similarity index 100%",
    "rename from old.txt",
    "rename to new.txt",
    "diff --git a/source.txt b/copied.txt",
    "similarity index 100%",
    "copy from source.txt",
    "copy to copied.txt",
    "diff --git a/script.sh b/script.sh",
    "old mode 100644",
    "new mode 100755",
    "diff --git a/logo.png b/logo.png",
    "index 1111111..2222222 100644",
    "Binary files a/logo.png and b/logo.png differ",
  ].join("\n");
  const parsed = parsePatch(raw);

  assert.deepEqual(parsed.files.map(({ path, status, additions, deletions }) => (
    { path, status, additions, deletions }
  )), [
    { path: "new.txt", status: "renamed", additions: 0, deletions: 0 },
    { path: "copied.txt", status: "copied", additions: 0, deletions: 0 },
    { path: "script.sh", status: "mode-only", additions: 0, deletions: 0 },
    { path: "logo.png", status: "binary", additions: null, deletions: null },
  ]);
  assert.equal(parsed.totals.available, false);
  assert.equal(parsed.totals.additions, null);
  assertLossless(parsed);
});

test("malformed hunks are unassigned without corrupting later files", () => {
  const raw = [
    "diff --git a/bad.txt b/bad.txt",
    "index 1111111..2222222 100644",
    "--- a/bad.txt",
    "+++ b/bad.txt",
    "@@ -1,2 +1,2 @@",
    " only-one",
    ordinaryFile("good.txt", 70, 80),
  ].join("\n");
  const parsed = parsePatch(raw);

  assert.equal(parsed.files[0].countsAvailable, false);
  assert.equal(parsed.files[0].confidence, "partial");
  assert.deepEqual(parsed.files.slice(1).map(({ additions, deletions, confidence }) => (
    { additions, deletions, confidence }
  )), [{ additions: 1, deletions: 1, confidence: "full" }]);
  assert.equal(parsed.confidence, "partial");
  assert.deepEqual(parsed.totals, { files: 2, additions: null, deletions: null, available: false });
  assert.ok(parsed.unassigned.every((region) => raw.slice(region.from, region.to).length > 0));
  assertLossless(parsed);
});

test("warnings before, between and after files remain unassigned and visible in file slices", () => {
  const first = ordinaryFile("one.txt", 1, 1);
  const second = ordinaryFile("two.txt", 2, 2);
  const raw = `leading warning\n${first}\nbetween warning\n${second}\ntrailing warning\n`;
  const parsed = parsePatch(raw);

  assert.equal(parsed.available, true);
  assert.equal(parsed.confidence, "partial");
  assert.match(raw.slice(parsed.files[0].from, parsed.files[0].to), /between warning/);
  assert.match(raw.slice(parsed.files[1].from, parsed.files[1].to), /trailing warning/);
  assert.equal(parsed.unassigned.map((region) => raw.slice(region.from, region.to)).join(""),
    "leading warning\nbetween warning\ntrailing warning\n");
  assert.deepEqual(parsed.totals, { files: 2, additions: null, deletions: null, available: false });
  assertLossless(parsed);
});

test("all-repository headers alone prove clean zero-change repositories", () => {
  const raw = "\n=== src/one/ ===\n\n=== src/two/ ===\n";
  const parsed = parsePatch(raw);

  assert.equal(parsed.available, true);
  assert.deepEqual(parsed.repositories.map(({ name, zeroChange, confidence }) => (
    { name, zeroChange, confidence }
  )), [
    { name: "one", zeroChange: true, confidence: "full" },
    { name: "two", zeroChange: true, confidence: "full" },
  ]);
  assert.deepEqual(parsed.totals, { files: 0, additions: 0, deletions: 0, available: true });
  assertLossless(parsed);
});

test("a repository header followed only by command output does not claim a structured view", () => {
  const raw = "\n=== src/broken/ ===\nfatal: ambiguous argument\n";
  const parsed = parsePatch(raw);

  assert.equal(parsed.available, false);
  assert.equal(parsed.confidence, "none");
  assert.deepEqual(parsed.repositories.map(({ zeroChange, confidence }) => ({ zeroChange, confidence })), [
    { zeroChange: false, confidence: "partial" },
  ]);
  assert.deepEqual(parsed.totals, { files: 0, additions: null, deletions: null, available: false });
  assertLossless(parsed);
});

test("deleted files use the old path and validated deletion count", () => {
  const raw = [
    "diff --git a/gone.txt b/gone.txt",
    "deleted file mode 100644",
    "index 1111111..0000000",
    "--- a/gone.txt",
    "+++ /dev/null",
    "@@ -1,2 +0,0 @@",
    "-first",
    "-second",
  ].join("\n");
  const parsed = parsePatch(raw);

  assert.deepEqual(parsed.files.map(({ path, status, additions, deletions }) => (
    { path, status, additions, deletions }
  )), [{ path: "gone.txt", status: "deleted", additions: 0, deletions: 2 }]);
  assert.deepEqual(parsed.totals, { files: 1, additions: 0, deletions: 2, available: true });
  assertLossless(parsed);
});

test("fatal, combined, malformed and ANSI-obscured boundaries have no structured view", () => {
  const cases = [
    "fatal: not a git repository\n",
    "diff --cc merge.txt\n@@@ -1,1 -1,1 +1,1 @@@\n combined\n",
    "diff --git a/one.txt\n@@ -1 +1 @@\n-old\n+new\n",
    "\u001b[1mdiff --git a/one.txt b/one.txt\u001b[0m\n@@ -1 +1 @@\n-old\n+new\n",
  ];
  for (const raw of cases) {
    const parsed = parsePatch(raw);
    assert.equal(parsed.available, false);
    assert.equal(parsed.confidence, "none");
    assert.equal(parsed.files.length, 0);
    assert.deepEqual(parsed.regions.map((region) => region.kind), ["unassigned"]);
    assertLossless(parsed);
  }
});

test("quoted paths decode valid Git bytes and preserve undecodable escapes visibly", () => {
  const raw = [
    "diff --git \"a/caf\\303\\251 name.txt\" \"b/caf\\303\\251 name.txt\"",
    "old mode 100644",
    "new mode 100755",
    "diff --git \"a/bad\\377.txt\" \"b/bad\\377.txt\"",
    "old mode 100644",
    "new mode 100755",
  ].join("\n");
  const parsed = parsePatch(raw);

  assert.equal(parsed.files[0].path, "café name.txt");
  assert.equal(parsed.files[1].path, "bad\\377.txt");
  assert.equal(parsed.confidence, "full");
  assertLossless(parsed);
});

test("UTF-16 offsets cover Unicode and CRLF exactly", () => {
  const raw = "\r\ndiff --git a/😀.txt b/😀.txt\r\nold mode 100644\r\nnew mode 100755\r\n";
  const parsed = parsePatch(raw);

  assert.equal(parsed.files[0].from, 2);
  assert.equal(parsed.files[0].to, raw.length);
  assert.equal(raw.slice(parsed.files[0].from, parsed.files[0].to), raw.slice(2));
  assert.equal(parsed.files[0].path, "😀.txt");
  assertLossless(parsed);
});

test("large safe hunk coordinates count exactly while unsafe coordinates make totals unavailable", () => {
  const safe = parsePatch([
    "diff --git a/safe b/safe",
    "--- a/safe",
    "+++ b/safe",
    "@@ -9007199254740991 +0,0 @@",
    "-last",
  ].join("\n"));
  assert.deepEqual(safe.totals, { files: 1, additions: 0, deletions: 1, available: true });

  const unsafe = parsePatch([
    "diff --git a/unsafe b/unsafe",
    "--- a/unsafe",
    "+++ b/unsafe",
    "@@ -9007199254740992 +1 @@",
    "-old",
    "+new",
  ].join("\n"));
  assert.deepEqual(unsafe.totals, { files: 1, additions: null, deletions: null, available: false });
  assert.equal(unsafe.confidence, "partial");
  assertLossless(safe);
  assertLossless(unsafe);
});
