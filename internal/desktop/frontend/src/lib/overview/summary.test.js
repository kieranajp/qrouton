import assert from "node:assert/strict";
import test from "node:test";
import { summaryMeta } from "./summary.js";

const now = Date.parse("2026-10-11T15:00:00Z");
const summary = (extra = {}) => ({ text: "Fix flaky refund webhook test.", stage: "plan", at: "2026-10-11T14:56:00Z", ...extra });

test("a settled summary names the lead it was written for and its age", () => {
  assert.deepEqual(summaryMeta(summary({ stage: "research" }), now).meta, "As of Research · 4m ago");
  assert.deepEqual(summaryMeta(summary({ stage: "plan" }), now).meta, "As of Plan · 4m ago");
  assert.deepEqual(summaryMeta(summary({ stage: "implement" }), now), {
    text: "Fix flaky refund webhook test.",
    meta: "As of Implementation · 4m ago",
    empty: false,
  });
});

test("a summary written this minute reads now, without ago", () => {
  assert.equal(summaryMeta(summary({ at: "2026-10-11T14:59:40Z" }), now).meta, "As of Plan · just now");
});

test("a summary written before any repository shows only its age", () => {
  assert.equal(summaryMeta(summary({ stage: "" }), now).meta, "4m ago");
});

test("no summary, or a blank one, is the empty state", () => {
  const empty = { text: "No summary yet", meta: "Written when the first lead starts", empty: true };
  assert.deepEqual(summaryMeta(undefined, now), empty);
  assert.deepEqual(summaryMeta({ text: "", stage: "", at: "" }, now), empty);
  assert.deepEqual(summaryMeta(summary({ text: "  " }), now), empty);
});
