import assert from "node:assert/strict";
import { test } from "node:test";
import { workflowTone } from "./workflow.js";

test("workflow stages resolve their semantic tones through one API", () => {
  assert.equal(workflowTone("RESEARCH"), "var(--artifact-research)");
  assert.equal(workflowTone("PLAN"), "var(--artifact-plan)");
  assert.equal(workflowTone("IMPLEMENT"), "var(--state-success)");
});
