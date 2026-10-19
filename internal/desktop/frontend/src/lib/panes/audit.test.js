import assert from "node:assert/strict";
import { test } from "node:test";
import { byImpact, parseAudit, readTriage, reapply, spliceTriage } from "./audit.js";

const doc = (...lines) => lines.join("\n");

const AUDIT = doc(
  "---",
  "kind: audit",
  "title: Retry package",
  "target: lsx internal/retry",
  "---",
  "",
  "# Retry package",
  "",
  "## Summary",
  "",
  "Three findings; the timeout helpers matter most.",
  "",
  "## Duplicated behaviour or facts",
  "",
  "### F1 — Two timeout helpers race a promise against a timer",
  "",
  "Tell: duplicated behaviour or facts",
  "Locations: src/api/http.ts:41, src/sync/queue.ts:88",
  "Evidence: both wrap a promise in a race",
  "with a timer that rejects.",
  "Verdict: rot. Fails step 1, no tie: nothing fails if one copy drifts",
  "Proposed fix: keep the http helper and import it in sync.",
  "Behaviour risk: safe",
  "Impact: high",
  "Confidence: medium",
  "Source: jscpd",
  "",
  "Triage: reject",
  "the copy in sync/ is kept for the offline build",
  "",
  "## Dead or needlessly defensive code",
  "",
  "### F2 — A nil check on a value that is never nil",
  "",
  "Tell: dead or needlessly defensive code",
  "Locations: internal/retry/retry.go:12",
  "Evidence: the constructor always sets it.",
  "Verdict: fails step 3: the guarded case cannot occur",
  "Proposed fix: drop the check.",
  "Behaviour risk: safe",
  "Impact: low",
  "Confidence: high",
  "Source: llm",
  "",
  "Triage: accept, but after the release",
  "",
  "### F3 — A backoff override only tests call",
  "",
  "Tell: dead or needlessly defensive code",
  "Locations: internal/retry/backoff.go:30",
  "Evidence: every caller is a test.",
  "Verdict: rot. Fails step 5: nothing shipped reaches it",
  "Proposed fix: drop the override.",
  "Behaviour risk: needs a test first",
  "Confidence: low",
  "Source: llm",
  "",
  "## Homegrown library or platform behaviour",
  "",
  "None.",
  "",
  "## Checked and deliberate",
  "",
  "- internal/retry/jitter.go:8 — seeded for tests.",
  "",
  "## Tools",
  "",
  "- jscpd 4.0.5",
  "",
);

test("an audit's findings carry their tell section and fields", () => {
  const audit = parseAudit(AUDIT);
  assert.equal(audit.isAudit, true);
  assert.equal(audit.title, "Retry package");
  assert.deepEqual(audit.findings.map((finding) => finding.id), ["F1", "F2", "F3"]);
  assert.deepEqual(audit.tells.map((tell) => [tell.name, tell.count]), [
    ["Duplicated behaviour or facts", 1],
    ["Dead or needlessly defensive code", 2],
    ["Homegrown library or platform behaviour", 0],
  ]);
  const [f1] = audit.findings;
  assert.equal(f1.title, "Two timeout helpers race a promise against a timer");
  assert.equal(f1.section, "Duplicated behaviour or facts");
  assert.deepEqual(f1.fields.locations, ["src/api/http.ts:41", "src/sync/queue.ts:88"]);
  assert.equal(f1.fields.evidence, "both wrap a promise in a race\nwith a timer that rejects.");
  assert.equal(f1.fields.confidence, "medium");
  assert.equal(f1.fields.risk, "safe");
  assert.equal(f1.fields.impact, "high");
  assert.equal(audit.findings[2].fields.impact, "");
  assert.equal(f1.fields.source, "jscpd");
  assert.equal(f1.step, 1);
  assert.equal(audit.findings[2].step, 5);
  assert.deepEqual(audit.sections.map((section) => section.name), ["Summary", "Checked and deliberate", "Tools"]);
});

test("a document with no tell section is not an audit", () => {
  assert.equal(parseAudit(doc("# Notes", "", "## Summary", "", "Nothing.", "")).isAudit, false);
});

test("a lone choice on the Triage line is a choice, and the lines below are its note", () => {
  const [f1, f2] = parseAudit(AUDIT).findings;
  assert.deepEqual([f1.triage.choice, f1.triage.note], ["reject", "the copy in sync/ is kept for the offline build"]);
  assert.deepEqual([f2.triage.choice, f2.triage.note], ["", "accept, but after the release"]);
  assert.deepEqual(readTriage("maybe", ""), { choice: "", note: "maybe" });
  assert.deepEqual(readTriage(" defer ", ""), { choice: "defer", note: "" });
  assert.deepEqual(readTriage("", "accept"), { choice: "", note: "accept" });
  assert.deepEqual(readTriage("Accept", ""), { choice: "", note: "Accept" });
});

test("a finding with no Triage line has none, and splicing adds it last in the block", () => {
  const f3 = parseAudit(AUDIT).findings[2];
  assert.equal(f3.triage, null);
  const after = spliceTriage(AUDIT, f3, { choice: "defer", note: "" });
  assert.match(after, /Source: llm\n\nTriage: defer\n\n## Homegrown/);
  const read = parseAudit(after).findings[2].triage;
  assert.deepEqual([read.choice, read.note], ["defer", ""]);
});

test("splicing triage touches only that finding's triage block", () => {
  const audit = parseAudit(AUDIT);
  const after = spliceTriage(AUDIT, audit.findings[0], { choice: "accept", note: "line one\nline two" });
  const before = AUDIT.split("\n");
  const lines = after.split("\n");
  const { from, to } = audit.findings[0].triage;
  assert.deepEqual(lines.slice(0, from - 1), before.slice(0, from - 1));
  assert.deepEqual(lines.slice(from - 1 + 3), before.slice(to));
  const read = parseAudit(after).findings[0].triage;
  assert.deepEqual([read.choice, read.note], ["accept", "line one\nline two"]);
});

test("every triage form reads back as it was written", () => {
  const forms = [
    { choice: "accept", note: "" },
    { choice: "reject", note: "kept for the offline build" },
    { choice: "", note: "" },
    { choice: "", note: "maybe" },
    { choice: "", note: "accept, but after the release" },
    { choice: "", note: "accept" },
    { choice: "", note: "defer\nnot a choice" },
  ];
  for (const form of forms) {
    const finding = parseAudit(AUDIT).findings[1];
    const after = spliceTriage(AUDIT, finding, form);
    const read = parseAudit(after).findings[1].triage;
    assert.deepEqual([read.choice, read.note], [form.choice, form.note], JSON.stringify(form));
    assert.equal(spliceTriage(after, parseAudit(after).findings[1], { choice: read.choice, note: read.note }), after);
  }
});

test("findings sort by impact, then confidence, then document order, with unrated impact last", () => {
  const ids = (text) => byImpact(parseAudit(text).findings).map((finding) => finding.id);
  assert.deepEqual(ids(AUDIT), ["F1", "F2", "F3"]);
  assert.deepEqual(ids(AUDIT.replace("Impact: low", "Impact: high")), ["F2", "F1", "F3"]);
  assert.deepEqual(ids(AUDIT.replace("Confidence: low", "Confidence: high")), ["F1", "F2", "F3"]);
  assert.deepEqual(ids(AUDIT.replace("Impact: high", "Impact: low")), ["F2", "F1", "F3"]);
});

test("a retitled finding keeps its unsaved triage, and a removed one detaches", () => {
  const drafts = new Map([
    ["F1", { id: "F1", choice: "accept", note: "" }],
    ["F9", { id: "F9", choice: "defer", note: "gone" }],
  ]);
  const retitled = AUDIT.replace("### F1 — Two timeout helpers race a promise against a timer", "### F1 — One timeout helper too many");
  const { findings, detached } = reapply(parseAudit(retitled), drafts);
  assert.equal(findings[0].finding.title, "One timeout helper too many");
  assert.equal(findings[0].draft.choice, "accept");
  assert.equal(findings[1].draft, null);
  assert.deepEqual(detached.map((draft) => draft.id), ["F9"]);
});

test("ids stay with their findings when the agent inserts one above", () => {
  const inserted = AUDIT.replace(
    "### F2 — A nil check",
    doc("### F4 — A new finding", "", "Tell: dead or needlessly defensive code", "Confidence: low", "", "Triage:", "", "### F2 — A nil check"),
  );
  const audit = parseAudit(inserted);
  assert.deepEqual(audit.findings.map((finding) => finding.id), ["F1", "F4", "F2", "F3"]);
  assert.equal(audit.findings[2].title, "A nil check on a value that is never nil");
  assert.equal(audit.findings[1].triage.raw, "");
});

test("only the chosen tells yield findings, and an unchosen tell section reads as prose", () => {
  const audit = parseAudit(AUDIT, ["Dead or needlessly defensive code"]);
  assert.equal(audit.isAudit, true);
  assert.deepEqual(audit.findings.map((finding) => finding.id), ["F2", "F3"]);
  assert.deepEqual(audit.tells.map((tell) => [tell.name, tell.count]), [["Dead or needlessly defensive code", 2]]);
  assert.deepEqual(audit.sections.map((section) => section.name), [
    "Summary",
    "Duplicated behaviour or facts",
    "Homegrown library or platform behaviour",
    "Checked and deliberate",
    "Tools",
  ]);
});

test("a chosen tell with no section counts 0, in the chosen order", () => {
  const without = AUDIT.replace("## Homegrown library or platform behaviour\n\nNone.\n\n", "");
  const audit = parseAudit(without, ["Dead or needlessly defensive code", "Homegrown library or platform behaviour"]);
  assert.deepEqual(audit.tells.map((tell) => [tell.name, tell.count]), [
    ["Dead or needlessly defensive code", 2],
    ["Homegrown library or platform behaviour", 0],
  ]);
});

test("no chosen tells means every tell section present", () => {
  const without = AUDIT.replace("## Homegrown library or platform behaviour\n\nNone.\n\n", "");
  assert.deepEqual(parseAudit(without, null).tells.map((tell) => tell.name), [
    "Duplicated behaviour or facts",
    "Dead or needlessly defensive code",
  ]);
  assert.deepEqual(parseAudit(AUDIT, null), parseAudit(AUDIT));
});

test("a tell-4 finding parses and names its step", () => {
  const withTell4 = AUDIT.replace(
    "## Checked and deliberate",
    doc(
      "## Tests that pass or fail for the wrong reason",
      "",
      "### F4 — A test asserts the stub it configured",
      "",
      "Tell: tests that cannot fail or pass for the wrong reason",
      "Verdict: rot. Fails step 7: the test cannot fail",
      "",
      "Triage:",
      "",
      "## Checked and deliberate",
    ),
  );
  const audit = parseAudit(withTell4);
  const f4 = audit.findings.find((finding) => finding.id === "F4");
  assert.equal(f4.section, "Tests that pass or fail for the wrong reason");
  assert.equal(f4.step, 7);
  const last = audit.tells.at(-1);
  assert.deepEqual([last.name, last.count], [f4.section, 1]);
});

test("a step-10 verdict names step 10, not step 1", () => {
  const audit = parseAudit(AUDIT.replace("Fails step 1, no tie", "Fails step 10, no tie"));
  assert.equal(audit.findings[0].step, 10);
});

test("an old tell-4-only audit is an audit when Go chooses its old heading", () => {
  const old = "Tests that cannot fail or pass for the wrong reason";
  const text = doc(
    "# Old audit",
    "",
    "## Summary",
    "",
    "One finding.",
    "",
    `## ${old}`,
    "",
    "### F1 — A skipped test",
    "",
    "Verdict: rot. Fails step 6",
    "",
    "Triage:",
  );
  assert.equal(parseAudit(text).isAudit, false);
  const audit = parseAudit(text, [old]);
  assert.equal(audit.isAudit, true);
  assert.deepEqual(audit.findings.map((finding) => [finding.id, finding.section, finding.step]), [["F1", old, 6]]);
});
