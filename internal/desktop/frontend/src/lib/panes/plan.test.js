import assert from "node:assert/strict";
import { test } from "node:test";
import { criteriaSpans, parsePlan } from "./plan.js";

const doc = (...lines) => lines.join("\n");

const PLAN = doc(
  "---",
  "kind: plan",
  "title: Phase work",
  "---",
  "",
  "# The deck",
  "",
  "Prose before anything opens.",
  "",
  "## Phase 1 — The parser",
  "",
  "Body copy.",
  "",
  "### Verify",
  "- [x] one",
  "- [ ] two",
  "",
  "## Phase 2: The deck",
  "",
  "### See",
  "- [x] the pane draws",
  "",
  "### Verify",
  "- [ ] `npm run check`",
  "- [~] superseded",
  "",
  "## Phase 3 — Everything",
  "",
  "### Verify",
  "- [x] done",
  "- [x] also done",
  "",
);

test("both separators open a phase", () => {
  const { phases } = parsePlan(PLAN);
  assert.deepEqual(
    phases.map((phase) => [phase.index, phase.name]),
    [
      [1, "The parser"],
      [2, "The deck"],
      [3, "Everything"],
    ],
  );
});

test("the meter counts ticks in the criteria list", () => {
  const [first, , third] = parsePlan(PLAN).phases;
  assert.deepEqual([first.met, first.total, first.state], [1, 2, "working"]);
  assert.deepEqual([third.met, third.total, third.state], [2, 2, "met"]);
});

test("a marker GFM does not recognise is a criterion and is unmet", () => {
  const [, second] = parsePlan(PLAN).phases;
  assert.deepEqual(
    second.criteria.map((criterion) => [criterion.text, criterion.met]),
    [
      ["npm run check", false],
      ["superseded", false],
    ],
  );
  assert.equal(second.state, "not-started");
});

test("a fully ticked See list neither counts nor moves the state", () => {
  const [, second] = parsePlan(PLAN).phases;
  assert.equal(second.total, 2);
  assert.equal(second.met, 0);
  assert.ok(!second.criteria.some((criterion) => criterion.text === "the pane draws"));
});

test("a phase whose See list is ticked and whose criteria are not reads working", () => {
  const { phases } = parsePlan(
    doc(
      "## Phase 1 — Mixed",
      "",
      "### See",
      "- [x] looks right",
      "- [x] reads right",
      "",
      "### Verify",
      "- [x] one",
      "- [ ] two",
      "",
    ),
  );
  assert.deepEqual([phases[0].met, phases[0].total, phases[0].state], [1, 2, "working"]);
});

test("a phase with observations and no criteria states nothing", () => {
  const { phases } = parsePlan(doc("## Phase 1 — Bare", "", "### See", "- [x] looks right", ""));
  assert.deepEqual([phases[0].met, phases[0].total, phases[0].state], [0, 0, "not-started"]);
});

test("phase spans run to the line before the next opens", () => {
  const { title, preamble, phases } = parsePlan(PLAN);
  assert.equal(title, "The deck");
  assert.deepEqual(preamble, { from: 5, to: 9 });
  assert.deepEqual(
    phases.map((phase) => [phase.from, phase.to]),
    [
      [10, 17],
      [18, 26],
      [27, 32],
    ],
  );
});

test("the criteria span covers the heading and its list alone", () => {
  const [first, second] = parsePlan(PLAN).phases;
  assert.deepEqual(criteriaSpans(first), { from: 14, to: 16 });
  assert.deepEqual(criteriaSpans(second), { from: 23, to: 25 });
});

test("a document nothing opens a phase in has none", () => {
  const { phases } = parsePlan(doc("# Notes", "", "## Background", "", "- [x] a ticked box", ""));
  assert.deepEqual(phases, []);
  assert.equal(criteriaSpans(undefined), null);
});

test("frontmatter naming a phase opens nothing", () => {
  const { phases, preamble } = parsePlan(
    doc("---", "title: Phase 4 — the leftovers", "---", "", "# Notes", "", "Prose.", ""),
  );
  assert.deepEqual(phases, []);
  assert.equal(preamble.from, 4);
});
