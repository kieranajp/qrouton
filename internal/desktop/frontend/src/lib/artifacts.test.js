import assert from "node:assert/strict";
import { test } from "node:test";
import { artifactTone } from "./artifacts.js";

test("each artifact kind resolves to its design token", () => {
  assert.equal(artifactTone("PLAN"), "var(--artifact-plan)");
  assert.equal(artifactTone("SPEC"), "var(--artifact-spec)");
  assert.equal(artifactTone("RESEARCH"), "var(--artifact-research)");
  assert.equal(artifactTone("NOTE"), "var(--artifact-note)");
  assert.equal(artifactTone("unknown"), "var(--artifact-note)");
});
