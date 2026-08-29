import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import ResearchFixture from "./ResearchFixture.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

export const RESEARCH = [
  "---", // 1
  "kind: research", // 2
  "---", // 3
  "", // 4
  "# The fixture research", // 5
  "", // 6
  "Preamble prose about what was investigated.", // 7
  "", // 8
  "## Summary", // 9
  "", // 10
  "What it all came to, in one paragraph.", // 11
  "", // 12
  "## How does the loader stamp a skill?", // 13
  "", // 14
  "It walks the folder and writes every file it finds.", // 15
  "", // 16
  "```go", // 17
  "func main() {}", // 18
  "```", // 19
  "", // 20
  "## Where does the kind come from?", // 21
  "", // 22
  "From the path segment, or the filename prefix.", // 23
  "", // 24
  "| left | right |", // 25
  "| ---- | ----- |", // 26
  "| 1    | 2     |", // 27
  "", // 28
  "## Open Questions", // 29
  "", // 30
  "Nothing outstanding.", // 31
  "",
].join("\n");

// The same document before anyone has answered it: the summary frames what is
// being investigated and each question still holds only the context a
// researcher was given.
export const UNANSWERED = [
  "---",
  "kind: research",
  "---",
  "",
  "# The fixture research",
  "",
  "## Summary",
  "",
  "What is being looked at, and where to start.",
  "",
  "## How does the loader stamp a skill?",
  "",
  "> Start in prompts/loader.go.",
  "",
  "## Where does the kind come from?",
  "",
  "> Start in internal/status.",
  "",
].join("\n");

const PLAIN = ["# Just notes", "", "No headings open anything here.", "", "More prose."].join("\n");

const params = new URLSearchParams(location.search);
const number = (name) => Number(params.get(name) ?? 0);

const document_ = (text, epoch) => ({
  text,
  format: "markdown",
  source: "thoughts/shared/research/R001-fixture.md",
  path: "/sessions/fixture/thoughts/shared/research/R001-fixture.md",
  kind: "RESEARCH",
  line: number("line"),
  to: number("to"),
  viewportEpoch: epoch,
});

window.reports = [];
window.wailsCall = async (name, id, payload) => {
  if (name.endsWith(".Content")) {
    const text = params.get("plain") ? PLAIN : params.get("unanswered") ? UNANSWERED : RESEARCH;
    return document_(text, 1);
  }
  if (name.endsWith(".RenderDiagrams")) return [];
  if (name.endsWith(".ReportViewport")) window.reports.push(payload);
  return undefined;
};

// The push the workbench sends when the file changes under an open tab.
window.pushContent = (fields) =>
  emitWailsEvent("window:content:w1", { ...document_(RESEARCH, 1), ...fields });
window.pushEdited = () =>
  window.pushContent({
    text: RESEARCH.replace("It walks the folder", "It now walks the folder"),
  });

window.errors = [];
addEventListener("error", (event) => window.errors.push(String(event.message)));

const items = () => [...document.querySelectorAll(".item")];
// Every accordion row and whether its body is on screen, in document order.
window.items = () =>
  items().map((item) => ({ name: item.dataset.item, open: item.open }));
window.opened = () =>
  items()
    .filter((item) => item.open)
    .map((item) => item.dataset.item);
window.markedLines = () =>
  [...document.querySelectorAll(".marked")].map((el) => Number(el.dataset.line));
window.mode = () => document.querySelector('[aria-pressed="true"]').textContent.trim();
window.counter = () => document.querySelector(".counter").textContent.replace(/\s+/g, " ").trim();
// A row counts as drawn when the blocks the viewport measures have a box.
window.drawnItems = () =>
  items()
    .filter((item) =>
      [...item.querySelectorAll("[data-line]")].some((block) => {
        const box = block.getBoundingClientRect();
        return box.width > 0 && box.height > 0;
      }),
    )
    .map((item) => item.dataset.item);

mount(ResearchFixture, { target: document.querySelector("#fixture") });
