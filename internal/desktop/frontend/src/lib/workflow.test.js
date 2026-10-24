import assert from "node:assert/strict";
import { test } from "node:test";
import { stageTone, workflowTone } from "./workflow.js";

test("workflow stages resolve their semantic tones through one API", () => {
  assert.equal(workflowTone("RESEARCH"), "var(--artifact-research)");
  assert.equal(workflowTone("PLAN"), "var(--artifact-plan)");
  assert.equal(workflowTone("IMPLEMENT"), "var(--state-success)");
});

test("an agent stage value resolves to its workflow tone", () => {
  assert.equal(stageTone("research"), workflowTone("RESEARCH"));
  assert.equal(stageTone("plan"), workflowTone("PLAN"));
  assert.equal(stageTone("implement"), workflowTone("IMPLEMENT"));
});

test("an unknown or empty stage has no workflow tone", () => {
  assert.equal(stageTone("triage"), undefined);
  assert.equal(stageTone(""), undefined);
  assert.equal(stageTone(undefined), undefined);
});
