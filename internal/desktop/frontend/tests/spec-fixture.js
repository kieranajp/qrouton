import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import SpecFixture from "./SpecFixture.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

export const OPEN = [
  "---",
  "kind: spec",
  "---",
  "",
  "# Retry timeouts",
  "",
  "Callers need a bound on how long a retry runs.",
  "",
  "## End state",
  "",
  "A caller can bound a retry.",
  "",
  "## Open questions",
  "",
  "### Q1 — Per attempt or overall?",
  "",
  "Every caller depends on this.",
  "",
  "- A. Per attempt",
  "- B. Overall **(recommended: it caps the caller's wait)**",
  "- C. Both",
  "",
  "Answer: B",
  "",
  "### Q2 — Which error comes back?",
  "",
  "- A. The last attempt's error",
  "- B. The context's error **(recommended: callers can test for it)**",
  "",
  "Answer:",
  "",
  "### Q3 — Is zero attempts an error?",
  "",
  "- A. Yes",
  "- B. No",
  "",
  "Answer:",
  "",
  "## Decisions",
  "",
  "**Keep `Retry` as a wrapper.** Nobody breaks.",
  "Rejected: changing its signature.",
  "",
  "**Cancellation is cooperative.** No goroutine is abandoned.",
  "",
  "## Risks",
  "",
  "None.",
  "",
].join("\n");

export const ZERO = [
  "---",
  "kind: spec",
  "---",
  "",
  "# Account audit log",
  "",
  "## Open questions",
  "",
  "None.",
  "",
  "## Decisions",
  "",
  "**The service writes entries.** The store stays plain.",
  "",
  "**Append before put.** A failed append stops the change.",
  "",
].join("\n");

export const CLEF = [
  "---",
  "kind: spec",
  "---",
  "",
  "# Clef bands",
  "",
  "## Open questions",
  "",
  "### Q1 — How does the ring fill?",
  "",
  "- A. From bands",
  "- B. From grams",
  "",
  "Answer:",
  "",
  "## Decisions",
  "",
  "### Product",
  "",
  "**Grams go entirely.** Each nutrient gets one band.",
  "",
  "**The bars go.** Nothing reads them.",
  "",
  "**A failed estimate shows no dots.** Nothing is drawn.",
  "",
  "### The Clef call",
  "",
  "**nutrition-intelligence owns the call.** Through its api task.",
  "",
  "**lsx makes one call per entry.** It posts the text.",
  "",
  "### Storage",
  "",
  "**Facets carry bands.** A nutrients object.",
  "",
  "**Old grams are dropped.** Nothing converts them.",
  "",
].join("\n");

export const FREEFORM = [
  "# An older spec",
  "",
  "## Approach",
  "",
  "Do the thing the old way.",
  "",
  "## Decisions",
  "",
  "- use X",
  "",
].join("\n");

export const BULLETS = [
  "---",
  "kind: spec",
  "---",
  "",
  "# Retry budget",
  "",
  "## Open questions",
  "",
  "None.",
  "",
  "## Decisions",
  "",
  "- Retries stop after three attempts",
  "- Backoff is capped at ten seconds",
  "",
].join("\n");

const params = new URLSearchParams(location.search);
const initial = params.get("zero") ? ZERO : params.get("freeform") ? FREEFORM : params.get("clef") ? CLEF : params.get("bullets") ? BULLETS : OPEN;

// Any stable digest will do: the pane only compares what it was given.
const hashOf = (text) => {
  let h = 0;
  for (const c of text) h = (Math.imul(h, 31) + c.charCodeAt(0)) | 0;
  return `h${(h >>> 0).toString(16)}`;
};

window.file = initial;
let stale = null;

const document_ = (fields = {}) => ({
  text: window.file,
  hash: hashOf(window.file),
  format: "markdown",
  source: "thoughts/shared/specs/S1-fixture.md",
  path: "/sessions/fixture/thoughts/shared/specs/S1-fixture.md",
  kind: "SPEC",
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
  if (name.endsWith(".SaveSpec")) {
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
window.sends = () => window.calls.filter((call) => call.name.endsWith(".SendSpecAnswers")).map((call) => call.args);
// Saves hang until released, so a test can look at the pane mid-save.
let hold = null;
window.holdSaves = () => {
  hold = new Promise((resolve) => (window.releaseSaves = () => {
    hold = null;
    resolve();
  }));
};
window.saves = () => window.calls.filter((call) => call.name.endsWith(".SaveSpec")).map((call) => call.args);
window.OPEN = OPEN;

window.pushContent = (text) => {
  window.file = text;
  emitWailsEvent("window:content:w1", document_());
};

const screens = () => [...document.querySelectorAll("[data-screen]")];
window.shown = () =>
  screens()
    .filter((screen) => !screen.hasAttribute("hidden"))
    .map((screen) => screen.dataset.screen);
window.pips = () => [...document.querySelectorAll(".pip")].map((pip) => pip.getAttribute("aria-label"));
window.focusedCard = () => document.querySelector(".card.focused")?.dataset.question ?? "";

mount(SpecFixture, { target: document.querySelector("#fixture") });
