import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import PlanFixture from "./PlanFixture.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

export const PLAN = [
  "---", // 1
  "kind: plan", // 2
  "---", // 3
  "", // 4
  "# The fixture plan", // 5
  "", // 6
  "Preamble prose about the whole thing.", // 7
  "", // 8
  "## Phase 1 — Groundwork", // 9
  "", // 10
  "Groundwork body copy.", // 11
  "", // 12
  "### Verify", // 13
  "- [x] the first check", // 14
  "- [x] the second check", // 15
  "", // 16
  "## Phase 2 — The middle", // 17
  "", // 18
  "Middle body copy a span can point at.", // 19
  "", // 20
  "Another middle paragraph.", // 21
  "", // 22
  "```go", // 23
  "func main() {}", // 24
  "```", // 25
  "", // 26
  "| left | right |", // 27
  "| ---- | ----- |", // 28
  "| 1    | 2     |", // 29
  "", // 30
  "### See", // 31
  "- [x] the deck looks right", // 32
  "- [x] the pips line up", // 33
  "", // 34
  "### Verify", // 35
  "- [x] one check ticked", // 36
  "- [ ] one check not", // 37
  "", // 38
  "## Phase 3 — The end", // 39
  "", // 40
  "Closing body copy.", // 41
  "", // 42
  "### Verify", // 43
  "- [ ] nothing ticked yet", // 44
  "",
].join("\n");

// Every criterion ticked, so a pushed document can move the meter.
export const FINISHED = PLAN.replace("- [ ] one check not", "- [x] one check not").replace(
  "- [ ] nothing ticked yet",
  "- [x] nothing ticked yet",
);

const PLAIN = ["# Just notes", "", "No headings open anything here.", "", "- [x] a ticked box", ""].join("\n");

const params = new URLSearchParams(location.search);
const number = (name) => Number(params.get(name) ?? 0);

const document_ = (text, epoch) => ({
  text,
  format: "markdown",
  source: "thoughts/shared/plans/P001-fixture.md",
  path: "/sessions/fixture/thoughts/shared/plans/P001-fixture.md",
  kind: "PLAN",
  line: number("line"),
  to: number("to"),
  viewportEpoch: epoch,
});

window.reports = [];
window.wailsCall = async (name, id, payload) => {
  if (name.endsWith(".Content")) return document_(params.get("plain") ? PLAIN : PLAN, 1);
  if (name.endsWith(".RenderDiagrams")) return [];
  if (name.endsWith(".ReportViewport")) window.reports.push(payload);
  return undefined;
};

// The push the workbench sends when the file changes under an open tab.
window.pushContent = (fields) => emitWailsEvent("window:content:w1", { ...document_(PLAN, 1), ...fields });
window.pushFinished = () => window.pushContent({ text: FINISHED });
window.pushEdited = () =>
  window.pushContent({ text: PLAN.replace("Another middle paragraph.", "An edited middle paragraph.") });

const screens = () => [...document.querySelectorAll("[data-screen]")];
window.shown = () =>
  screens()
    .filter((screen) => !screen.hasAttribute("hidden"))
    .map((screen) => screen.dataset.screen);
window.markedLines = () =>
  [...document.querySelectorAll(".marked")].map((el) => Number(el.dataset.line));
// A screen counts as drawn when the blocks the viewport measures have a box.
window.drawnScreens = () =>
  screens()
    .filter((screen) =>
      [...screen.querySelectorAll("[data-line]")].some((block) => {
        const box = block.getBoundingClientRect();
        return box.width > 0 && box.height > 0;
      }),
    )
    .map((screen) => screen.dataset.screen);
window.mode = () => (document.querySelector(".deck").hasAttribute("hidden") ? "raw" : "plan");
window.displays = () =>
  [...document.querySelectorAll("#scroll *")].map((el) => getComputedStyle(el).display);
window.headingSize = () =>
  Number.parseFloat(getComputedStyle(document.querySelector(".display-lg")).fontSize);
window.counters = () =>
  [...document.querySelectorAll(".rows .row")].map((row) => row.textContent.replace(/\s+/g, " ").trim());

mount(PlanFixture, { target: document.querySelector("#fixture") });
