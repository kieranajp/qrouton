import assert from "node:assert/strict";
import test from "node:test";
import { repositoryLine, rowLabel, summaryFacts } from "./activity.js";

test("summary facts preserve attention, agent, and unseen order", () => {
  const facts = summaryFacts(
    { attention: "unread", active: 3, coverage: "full", running: true },
    2,
  );
  assert.deepEqual(facts, [
    { kind: "attention", label: "Unread" },
    { kind: "agents", label: "3 active", active: true },
    { kind: "unseen", label: "2 new docs" },
  ]);
  assert.equal(rowLabel("Checkout", [{ name: "acme/web" }], facts),
    "Checkout · acme/web · Unread · 3 active · 2 new docs");
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
