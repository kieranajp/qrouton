import assert from "node:assert/strict";
import { test } from "node:test";
import { artifactInk, artifactLabel, artifactTone } from "./artifacts.js";

test("each artifact kind resolves to its design token", () => {
  assert.equal(artifactTone("PLAN"), "var(--artifact-plan)");
  assert.equal(artifactTone("SPEC"), "var(--artifact-spec)");
  assert.equal(artifactTone("RESEARCH"), "var(--artifact-research)");
  assert.equal(artifactTone("NOTE"), "var(--artifact-note)");
  assert.equal(artifactTone("EXPLAINER"), "var(--artifact-explainer)");
});

test("a kind no taxonomy claims takes the neutral block", () => {
  assert.equal(artifactTone("unknown"), "var(--surface-raised)");
  assert.equal(artifactInk("unknown"), "var(--text-secondary)");
  assert.equal(artifactLabel("unknown"), "DOC");
  assert.equal(artifactLabel(undefined, { long: true }), "DOCUMENT");
});

test("a hue carries crust text", () => {
  assert.equal(artifactInk("RESEARCH"), "var(--text-on-accent)");
});

test("the short label is the document's identity where it has one", () => {
  assert.equal(artifactLabel("RESEARCH", { id: "R1" }), "R1");
  assert.equal(artifactLabel("RESEARCH"), "RSCH");
  assert.equal(artifactLabel("RESEARCH", { id: "R1", long: true }), "RESEARCH");
});
