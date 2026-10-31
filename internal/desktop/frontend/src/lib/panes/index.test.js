import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { ARTIFACT_KINDS } from "../bridge/generated.js";
import { artifactLabel } from "../artifacts.js";

// The pane registry imports Svelte components, which node cannot load, so its
// kind table is read as text.
const registry = readFileSync(new URL("./index.js", import.meta.url), "utf8");
const paneKinds = new Set(
  [...registry.matchAll(/\[ARTIFACT_KINDS\.(\w+)\]:/g)].map((match) => match[1]),
);
const WITHOUT_PANE = new Set(["NOTE", "EXPLAINER", "REVIEW"]);

test("every artifact kind has a label", () => {
  for (const kind of Object.values(ARTIFACT_KINDS)) {
    assert.notEqual(artifactLabel(kind, { long: true }), "DOCUMENT", kind);
  }
});

test("every artifact kind but a note, explainer, or review has a pane", () => {
  for (const [key, kind] of Object.entries(ARTIFACT_KINDS)) {
    assert.equal(paneKinds.has(key), !WITHOUT_PANE.has(kind), kind);
  }
});
