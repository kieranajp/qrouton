import assert from "node:assert/strict";
import test from "node:test";
import {
  capabilityNote,
  parentLabel,
  projectAgents,
  providerLabel,
  recordLabel,
  roleLabel,
  rowLabel,
  stateLabel,
  summaryFacts,
  typeLabel,
} from "./activity.js";

test("summary facts preserve attention, agent, and unseen order", () => {
  const facts = summaryFacts(
    { attention: "needs-you", active: 3, coverage: "full", running: true },
    2,
  );
  assert.deepEqual(facts, [
    { kind: "attention", label: "Needs you" },
    { kind: "agents", label: "3 active" },
    { kind: "unseen", label: "2 unseen" },
  ]);
  assert.equal(rowLabel("Checkout", [{ name: "acme/web" }], facts),
    "Checkout · acme/web · Needs you · 3 active · 2 unseen");
});

test("unknown summary, role, state, provider, and type stay explicit", () => {
  assert.deepEqual(summaryFacts({ attention: "mystery", coverage: "mystery", running: true }), [
    { kind: "agents", label: "Activity unavailable" },
  ]);
  assert.equal(roleLabel("Coordinator"), "Role unavailable");
  assert.equal(stateLabel("Maybe"), "State unavailable");
  assert.equal(providerLabel(""), "Provider unknown");
  assert.equal(typeLabel(""), "Type unavailable");
  assert.equal(typeLabel("qrspi-planning-lead"), "QRSPI Planning Lead");
  assert.equal(parentLabel({}), "Parent unavailable");
  assert.equal(
    recordLabel({ role: "Specialist", type: "code-reviewer", state: "Finished" }),
    "Specialist · Code Reviewer · Finished",
  );
});

test("provider capability copy distinguishes root-only and missing integrations", () => {
  assert.equal(
    capabilityNote({ provider: "codex" }),
    "Codex provides root activity only. Attention, delegated agents, parent relationships, and outcomes unavailable.",
  );
  assert.equal(capabilityNote({}), "Provider unknown · live activity unavailable");
  assert.equal(
    capabilityNote({
      provider: "claude",
      attention_known: true,
      children_known: true,
      parents_known: false,
      outcomes_known: false,
    }),
    "Parent relationships and outcomes unavailable.",
  );
});

test("tree projection uses exact known parents and leaves unknown parentage flat", () => {
  const root = { id: "root", run_id: "7", provider: "claude", role: "Orchestrator" };
  const lead = {
    id: "lead-1", run_id: "7", provider: "claude", parent_id: "root", parent_known: true,
  };
  const specialist = {
    id: "specialist-1", run_id: "7", provider: "claude", parent_id: "lead-1", parent_known: true,
  };
  const unknown = { id: "agent-2", run_id: "7", provider: "claude", parent_known: false };
  const wrongRun = {
    id: "agent-3", run_id: "8", provider: "claude", parent_id: "lead-1", parent_known: true,
  };

  const projected = projectAgents([root, lead, specialist, unknown, wrongRun]);
  assert.equal(projected.trees.length, 1);
  assert.equal(projected.trees[0].children[0].record, lead);
  assert.equal(projected.trees[0].children[0].children[0].record, specialist);
  assert.deepEqual(projected.observed, [unknown, wrongRun]);
});
