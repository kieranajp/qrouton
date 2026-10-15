import "../src/tokens/scale.css";
import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import AuditFixture from "./AuditFixture.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

export const AUDIT = [
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
  "Triage:",
  "",
  "### F3 — A second clamp on the backoff",
  "",
  "Tell: dead or needlessly defensive code",
  "Locations: internal/retry/backoff.go:30",
  "Evidence: the caller clamps already.",
  "Verdict: fails step 3",
  "Proposed fix: drop the inner clamp.",
  "Behaviour risk: needs a test first",
  "Confidence: low",
  "Source: llm",
  "",
  "Triage:",
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
].join("\n");

// Any stable digest will do: the pane only compares what it was given.
const hashOf = (text) => {
  let h = 0;
  for (const c of text) h = (Math.imul(h, 31) + c.charCodeAt(0)) | 0;
  return `h${(h >>> 0).toString(16)}`;
};

window.file = AUDIT;
let stale = null;

const document_ = (fields = {}) => ({
  text: window.file,
  hash: hashOf(window.file),
  format: "markdown",
  source: "thoughts/shared/audits/A1-fixture.md",
  path: "/sessions/fixture/thoughts/shared/audits/A1-fixture.md",
  kind: "AUDIT",
  line: 0,
  to: 0,
  viewportEpoch: 1,
  ...fields,
});

window.calls = [];
window.wailsCall = async (name, ...args) => {
  window.calls.push({ name, args });
  if (name.endsWith(".Content")) return document_();
  if (name.endsWith(".RenderDiagrams")) return [];
  if (name.endsWith(".SaveAnswers")) {
    const [, hash, text] = args;
    if (hold) await hold;
    if (stale !== null) {
      window.file = stale;
      stale = null;
    }
    if (hash !== hashOf(window.file)) throw new Error("the document changed on disk since the pane read it");
    window.file = text;
    return hashOf(text);
  }
  return undefined;
};

// The next save finds the file already rewritten to this text.
window.staleOnce = (text) => (stale = text);
// Saves hang until released, so a test can look at the pane mid-save.
let hold = null;
window.holdSaves = () => {
  hold = new Promise((resolve) => (window.releaseSaves = () => {
    hold = null;
    resolve();
  }));
};
window.saves = () => window.calls.filter((call) => call.name.endsWith(".SaveAnswers")).map((call) => call.args);
window.sends = () => window.calls.filter((call) => call.name.endsWith(".SendAuditTriage")).map((call) => call.args);
window.AUDIT = AUDIT;

window.pushContent = (text) => {
  window.file = text;
  emitWailsEvent("window:content:w1", document_());
};

mount(AuditFixture, { target: document.querySelector("#fixture") });
