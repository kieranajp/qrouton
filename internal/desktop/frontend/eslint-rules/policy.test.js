import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { ESLint } from "eslint";
import { loadPolicy } from "./policy.js";

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

const valid = {
  schemaVersion: 1,
  maxCommentRun: 4,
  narrationPhrases: ["turns out"],
  pathExtensions: ["go", "js"],
};

test("policy loader accepts the shared schema", () => {
  assert.deepEqual(loadPolicy(writePolicy(valid)), valid);
});

test("policy loader rejects schema drift and invalid normalized values", () => {
  const cases = [
    { ...valid, schemaVersion: 2 },
    { ...valid, extra: true },
    { ...valid, maxCommentRun: 0 },
    { ...valid, narrationPhrases: [] },
    { ...valid, narrationPhrases: ["Turns out"] },
    { ...valid, pathExtensions: [".js"] },
    { ...valid, pathExtensions: ["js", "js"] },
  ];
  for (const value of cases) assert.throws(() => loadPolicy(writePolicy(value)));
});

test("policy loader rejects missing and malformed files", () => {
  assert.throws(() => loadPolicy(path.join(os.tmpdir(), `missing-policy-${process.pid}.json`)), /read policy/);
  const file = writeText("{");
  assert.throws(() => loadPolicy(file), /read policy/);
});

test("ESLint covers authored JavaScript and Svelte but not the generated bridge", async () => {
  const eslint = new ESLint({ cwd: frontendRoot });
  assert.equal(await eslint.isPathIgnored(path.join(frontendRoot, "src/main.js")), false);
  assert.equal(await eslint.isPathIgnored(path.join(frontendRoot, "src/Session.svelte")), false);
  assert.equal(await eslint.isPathIgnored(path.join(frontendRoot, "src/lib/bridge/generated.js")), true);
});

function writePolicy(value) {
  return writeText(JSON.stringify(value));
}

function writeText(value) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "comment-policy-"));
  const file = path.join(directory, "policy.json");
  fs.writeFileSync(file, value);
  return file;
}
