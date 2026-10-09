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

const params = new URLSearchParams(location.search);
const text = params.get("zero") ? ZERO : params.get("freeform") ? FREEFORM : OPEN;

const document_ = (fields = {}) => ({
  text,
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
  return undefined;
};

window.pushContent = (fields) => emitWailsEvent("window:content:w1", document_(fields));

const screens = () => [...document.querySelectorAll("[data-screen]")];
window.shown = () =>
  screens()
    .filter((screen) => !screen.hasAttribute("hidden"))
    .map((screen) => screen.dataset.screen);
window.pips = () => [...document.querySelectorAll(".pip")].map((pip) => pip.getAttribute("aria-label"));
window.focusedCard = () => document.querySelector(".card.focused")?.dataset.question ?? "";

mount(SpecFixture, { target: document.querySelector("#fixture") });
