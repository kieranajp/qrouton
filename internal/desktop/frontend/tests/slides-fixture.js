import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import SlidesFixture from "./SlidesFixture.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

export const DECK = [
  "---", // 1
  "marp: true", // 2
  "---", // 3
  "", // 4
  "<!-- _class: title -->", // 5
  "", // 6
  "# The fixture deck", // 7
  "", // 8
  "<!-- The note under the opener, which the reader can see. -->", // 9
  "", // 10
  "---", // 11
  "", // 12
  "## Second", // 13
  "", // 14
  "Body copy on the second slide.", // 15
  "", // 16
  "---", // 17
  "", // 18
  "## Third", // 19
  "", // 20
  "A slide with a very long note beneath it.", // 21
  "", // 22
  `<!-- ${"A long note that runs on and on. ".repeat(40)} -->`, // 23
  "", // 24
  "---", // 25
  "", // 26
  "## Fourth", // 27
  "", // 28
  "---", // 29
  "", // 30
  "## Fifth", // 31
  "", // 32
  "---", // 33
  "", // 34
  "## Sixth", // 35
  "", // 36
  "---", // 37
  "", // 38
  "## Seventh", // 39
  "", // 40
  "Body copy on the seventh slide.", // 41
  "", // 42
].join("\n");

export const SHORT_DECK = ["---", "marp: true", "---", "", "## Only", "", "---", "", "## Last", ""].join(
  "\n",
);

export const DIAGRAM_DECK = [
  "---", // 1
  "marp: true", // 2
  "---", // 3
  "", // 4
  "## Drawn", // 5
  "", // 6
  "```d2", // 7
  "a -> b", // 8
  "```", // 9
  "", // 10
].join("\n");
const DIAGRAM_LINE = 7;

// Shaped like d2's own output: a scaled size on the root beside a viewBox left
// at natural size.
const DIAGRAM_SVG =
  '<svg xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="xMinYMin meet"' +
  ' viewBox="0 0 1642 108" width="1067" height="70">' +
  '<rect width="1642" height="108" fill="#24273a"></rect>' +
  '<text x="40" y="60" fill="#cad3f5">a</text></svg>';

const params = new URLSearchParams(location.search);
const number = (name) => Number(params.get(name) ?? 0);

const document_ = (text) => ({
  text,
  format: "markdown",
  source: "thoughts/shared/decks/fixture.md",
  path: "/sessions/fixture/thoughts/shared/decks/fixture.md",
  kind: "NOTE",
  deck: true,
  assetToken: "tok",
  line: number("line"),
  to: number("to"),
  viewportEpoch: 1,
});

window.reports = [];
window.diagramReply = [];
window.pushDeck = (text) => emitWailsEvent("window:content:w1", document_(text));
window.shows = [];
window.opens = [];
window.closes = [];
window.wailsCall = async (name, ...args) => {
  if (name.endsWith(".Content")) return document_(DECK);
  if (name.endsWith(".RenderDiagrams")) return window.diagramReply;
  if (name.endsWith(".ReportViewport")) window.reports.push(args[1]);
  if (name.endsWith(".Show")) window.shows.push(args[0]);
  if (name.endsWith(".Open")) window.opens.push(name);
  if (name.endsWith(".Close")) window.closes.push(name);
  return undefined;
};
window.shorten = () => window.pushDeck(SHORT_DECK);

window.pushDiagramDeck = () => {
  window.diagramReply = [{ line: DIAGRAM_LINE }];
  window.pushDeck(DIAGRAM_DECK);
};
window.drawDiagram = () =>
  emitWailsEvent("window:diagram:w1", { line: DIAGRAM_LINE, svg: DIAGRAM_SVG });
window.failDiagram = (error) => emitWailsEvent("window:diagram:w1", { line: DIAGRAM_LINE, error });

window.diagram = () => {
  const block = document.querySelector(".card pre[data-line]");
  if (!block) return null;
  const drawn = block.querySelector("svg");
  const box = drawn?.getBoundingClientRect();
  const slide = block.closest("section").getBoundingClientRect();
  return {
    line: block.dataset.line,
    lineEnd: block.dataset.lineEnd,
    pending: block.classList.contains("diagram-pending"),
    drawn: block.classList.contains("diagram"),
    failed: block.classList.contains("diagram-failed"),
    zoomable: block.classList.contains("zoomable"),
    staged: Boolean(block.querySelector(".diagram-stage")),
    controls: block.querySelectorAll(".diagram-controls").length,
    error: block.querySelector(".diagram-error")?.textContent ?? "",
    code: Boolean(block.querySelector("code")),
    styleWidth: drawn?.style.width ?? "",
    attrWidth: drawn?.getAttribute("width") ?? "",
    viewBox: drawn?.getAttribute("viewBox") ?? "",
    width: box?.width ?? 0,
    height: box?.height ?? 0,
    slideWidth: slide.width,
    slideHeight: slide.height,
  };
};

const box = (element) => {
  const rect = element.getBoundingClientRect();
  return { width: rect.width, height: rect.height, top: rect.top };
};

window.cards = () =>
  [...document.querySelectorAll(".card")].map((card) => ({
    line: Number(card.dataset.line),
    lineEnd: Number(card.dataset.lineEnd),
    frame: box(card.querySelector(".frame")),
    notes: card.querySelector(".notes")?.getBoundingClientRect().height ?? 0,
  }));

window.counter = () => document.querySelector(".counter").textContent.trim();

window.slideScale = () =>
  Number(document.querySelector(".stack").style.getPropertyValue("--slide-scale"));

window.narrow = () => {
  document.querySelector("#fixture").style.width = "480px";
};

window.scroller = () => document.querySelector(".preview");

window.footerShape = () => {
  const footer = document.querySelector(".footer");
  const rect = footer.getBoundingClientRect();
  const pane = document.querySelector(".body");
  const bounds = pane.getBoundingClientRect();
  const scroller = window.scroller();
  return {
    height: rect.height,
    gap: bounds.bottom - rect.bottom,
    left: rect.left - bounds.left,
    width: rect.width,
    paneWidth: bounds.width,
    scrollerBottom: scroller.getBoundingClientRect().bottom,
    footerTop: rect.top,
    outerScroll: pane.scrollTop,
    outerOverflows: pane.scrollHeight > pane.clientHeight,
    innerOverflows: scroller.scrollHeight > scroller.clientHeight,
    pipRows: new Set([...footer.querySelectorAll(".pip")].map((pip) => pip.getBoundingClientRect().top)).size,
    controlsFit: [...footer.querySelectorAll("button")].every((button) => {
      const box = button.getBoundingClientRect();
      return box.left >= rect.left && box.right <= rect.right;
    }),
  };
};

const named = (label) =>
  [...document.querySelectorAll("button")].find((button) => button.textContent.trim() === label);

// Focused before the press, the way a real one lands, so the layer has
// something to hand the keyboard back to.
window.present = () => {
  const control = named("Present");
  control.focus();
  control.click();
};

window.notes = () => named("Notes").click();

// The presenter closing the second window from the OS rather than from the deck.
window.closeNotesWindow = () => emitWailsEvent("presenter:closed", null);

// Every app-level shortcut in this page is a window listener like this one.
window.appKeys = [];
window.addEventListener("keydown", (event) => window.appKeys.push(event.key));

window.slideBox = () => {
  const layer = document.querySelector(".present");
  const card = document.querySelector(".present-card");
  const slide = document.querySelector(".present-slide").getBoundingClientRect();
  const rect = card.getBoundingClientRect();
  const stage = document.querySelector(".present-stage").getBoundingClientRect();
  return {
    width: rect.width,
    height: rect.height,
    declared: document.querySelector(".present-slide").offsetWidth,
    scale: Number(card.style.getPropertyValue("--present-scale")),
    inset: {
      left: rect.left,
      top: rect.top,
      right: window.innerWidth - rect.right,
      bottom: window.innerHeight - rect.bottom,
    },
    // The card's padding box, which is what its overflow clips the slide to.
    // Whole pixels: getComputedStyle reports the border box instead.
    shows: { width: card.clientWidth, height: card.clientHeight },
    slide: { width: slide.width, height: slide.height },
    pad: parseFloat(getComputedStyle(layer).paddingTop),
    stage: { width: stage.width, height: stage.height },
    room: { width: window.innerWidth, height: window.innerHeight },
  };
};

window.layerHasFocus = () => document.activeElement?.classList.contains("present") === true;

window.presentHeading = () =>
  document.querySelector(".present-slide section h1, .present-slide section h2")?.textContent ?? "";

window.focusedLabel = () => document.activeElement?.textContent?.trim() ?? "";

mount(SlidesFixture, { target: document.querySelector("#fixture") });
