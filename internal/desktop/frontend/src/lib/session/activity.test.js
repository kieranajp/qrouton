import assert from "node:assert/strict";
import test from "node:test";
import {
  activeAgent,
  capabilityNote,
  duration,
  hierarchy,
  projectAgents,
  providerLabel,
  recordLabel,
  repositoryLine,
  roleLabel,
  rowLabel,
  stateLabel,
  subagentTally,
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
    { kind: "agents", label: "3 active", active: true },
    { kind: "unseen", label: "2 unseen" },
  ]);
  assert.equal(rowLabel("Checkout", [{ name: "acme/web" }], facts),
    "Checkout · acme/web · Needs you · 3 active · 2 unseen");
});

test("unsupported attention stays quiet while root activity remains explicit", () => {
  assert.deepEqual(
    summaryFacts({ attention: "unknown", active: 1, coverage: "root", running: true }),
    [{ kind: "agents", label: "Root active", active: true }],
  );
});

test("a session that is not running states how long it has been idle", () => {
  assert.deepEqual(summaryFacts({ running: false }, 0, "3d"), [
    { kind: "agents", label: "Idle · 3d" },
  ]);
  assert.deepEqual(summaryFacts({ running: false }), [{ kind: "agents", label: "Idle" }]);
});

test("a running session with unreadable coverage claims nothing about it", () => {
  assert.deepEqual(summaryFacts({ attention: "mystery", coverage: "mystery", running: true }), []);
});

test("a picker pending on the session yields its own fact", () => {
  assert.deepEqual(summaryFacts({ running: false }, 0, "", true), [
    { kind: "picker", label: "Repos requested" },
    { kind: "agents", label: "Idle" },
  ]);
  assert.deepEqual(summaryFacts({ running: false }, 0, "", false), [
    { kind: "agents", label: "Idle" },
  ]);
});

test("a name qrouton cannot read is a line it does not draw", () => {
  assert.equal(roleLabel("Coordinator"), "");
  assert.equal(stateLabel("Maybe"), "");
  assert.equal(providerLabel(""), "");
  assert.equal(typeLabel(""), "");
  assert.equal(typeLabel("qrspi-planning-lead"), "QRSPI Planning Lead");
  assert.equal(activeAgent({ state: "Active" }), true);
  assert.equal(activeAgent({ state: "Finished" }), false);
  assert.equal(
    recordLabel({ role: "Specialist", type: "code-reviewer", state: "Finished" }),
    "Specialist · Code Reviewer · Finished",
  );
});

test("waiting is the orchestrator's alone: only it can be blocked on the user", () => {
  assert.equal(stateLabel("Waiting for you", "Orchestrator"), "Waiting for you");
  assert.equal(stateLabel("Waiting for you", "Lead"), "Working");
  assert.equal(stateLabel("Waiting for you", "Specialist"), "Working");
});

test("one repository is named and the rest are counted", () => {
  assert.deepEqual(repositoryLine([{ name: "lifesum/lsx" }]), {
    name: "lifesum/lsx",
    extra: "",
  });
  assert.deepEqual(
    repositoryLine([{ name: "lifesum/gympass-users" }, { name: "lifesum/lsx" }]),
    { name: "lifesum/gympass-users", extra: "+1" },
  );
  assert.deepEqual(repositoryLine([]), { name: "No editing repositories", extra: "" });
});

test("a run with no start stamp behind it reports no duration", () => {
  const now = Date.parse("2026-09-01T12:00:00Z");
  assert.equal(duration({}, now), "");
  assert.equal(duration({ started_at: "2026-09-01T11:58:00Z" }, now), "2m");
  assert.equal(duration({ started_at: "2026-09-01T11:57:56Z" }, now), "2m 4s");
  assert.equal(duration({ started_at: "2026-09-01T11:59:20Z" }, now), "40s");
  assert.equal(duration({ started_at: "2026-09-01T10:57:00Z" }, now), "1h 3m");
  assert.equal(
    duration(
      { started_at: "2026-09-01T11:00:00Z", finished_at: "2026-09-01T11:05:00Z", state: "Finished" },
      now,
    ),
    "5m",
  );
});

test("the disclosure line counts the subagents and how many are through", () => {
  assert.equal(
    subagentTally([{ state: "Working" }, { state: "Finished" }, { state: "Failed" }]),
    "3 subagents · 2 done",
  );
  assert.equal(subagentTally([{ state: "Working" }]), "1 subagent · 0 done");
});

test("provider capability copy reports coverage without mentioning attention", () => {
  assert.equal(capabilityNote({ provider: "codex", children_known: true }), "");
  assert.equal(
    capabilityNote({ provider: "opencode", children_known: false }),
    "OpenCode provides root activity only.",
  );
  assert.equal(capabilityNote({}), "Provider unknown · live activity unavailable");
  assert.equal(
    capabilityNote({
      provider: "claude",
      children_known: true,
    }),
    "",
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

test("the hierarchy is three ranks, with subagents held behind their lead", () => {
  const root = { id: "root", run_id: "7", provider: "claude", role: "Orchestrator" };
  const lead = {
    id: "lead-1", run_id: "7", provider: "claude", parent_id: "root", parent_known: true,
    role: "Lead", type: "qrouton-planning-lead",
  };
  const sub = {
    id: "sub-1", run_id: "7", provider: "claude", parent_id: "lead-1", parent_known: true,
    role: "Specialist",
  };

  const ranks = hierarchy([root, lead, sub]);
  assert.equal(ranks.roots.length, 1);
  assert.equal(ranks.roots[0].record, root);
  assert.equal(ranks.roots[0].leads.length, 1);
  assert.equal(ranks.roots[0].leads[0].record, lead);
  assert.deepEqual(ranks.roots[0].leads[0].subagents, [sub]);
});
