import "../src/tokens/typography.css";
import "../src/lib/panes/markdown.css";
import { apply } from "../src/lib/panes/diagrams.js";
import { render } from "../src/lib/panes/markdown.js";

// One fence of each outcome, prose between them, numbered from the document.
const DOCUMENT = [
  "Before.",
  "",
  "```d2",
  "direction: right",
  "a -> b",
  "```",
  "",
  "Middle.",
  "",
  "```d2",
  "c: {",
  "```",
  "",
  "After.",
  "",
  "```d2",
  "a: |md",
  "  # heading",
  "|",
  "```",
  "",
  "Tail.",
  "",
  "```d2",
  "x -> y",
  "```",
  "",
  "End.",
  "",
];
const LINES = { drawn: 3, broken: 10, embedded: 16, narrow: 24 };
const TIMEOUT = "diagram took too long to lay out";
const root = document.querySelector("#markdown-root");

root.innerHTML = render(DOCUMENT.join("\n")).body;

// Shaped like d2's own output: the root carries the scaled size the renderer
// asked for beside a viewBox left at natural size.
const inner =
  '<svg class="d2-svg" width="1642" height="108" viewBox="-21 -21 1642 108">' +
  '<rect x="-21" y="-21" width="1642" height="108" fill="#24273a"></rect>' +
  '<text x="40" y="60" fill="#cad3f5">a</text></svg>';
const EMITTED = { width: 1067, height: 70 };
const svg =
  '<svg xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="xMinYMin meet"' +
  ` viewBox="0 0 1642 108" width="${EMITTED.width}" height="${EMITTED.height}">` +
  inner +
  "</svg>";
// Narrower than the fixture's container, so it opens at the emitted size.
const NARROW = { width: 180, height: 90 };
const narrow =
  '<svg xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="xMinYMin meet"' +
  ` viewBox="0 0 277 138" width="${NARROW.width}" height="${NARROW.height}">` +
  '<rect width="277" height="138" fill="#24273a"></rect>' +
  '<text x="20" y="60" fill="#cad3f5">x</text></svg>';
// Output from before the renderer carried a scale, which is sized by fallback.
const sizeless =
  '<svg xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="xMinYMin meet" viewBox="0 0 1642 108">' +
  inner +
  "</svg>";

window.lines = LINES;
window.emitted = EMITTED;
window.narrow = NARROW;
window.pending = () => apply(root, Object.values(LINES).map((line) => ({ line })));
window.draw = () => apply(root, [{ line: LINES.drawn, svg }]);
window.drawSizeless = () => apply(root, [{ line: LINES.drawn, svg: sizeless }]);
window.drawNarrow = () => apply(root, [{ line: LINES.narrow, svg: narrow }]);
window.fail = (line = LINES.drawn, error = TIMEOUT) => apply(root, [{ line, error }]);
window.settle = () =>
  apply(root, [
    { line: LINES.drawn, svg },
    { line: LINES.broken, error: "11:5: unexpected end of file" },
    { line: LINES.embedded, error: "diagram embeds HTML; |md| blocks are not rendered" },
  ]);
const stageOf = (line) => root.querySelector(`pre[data-line="${line}"] .diagram-stage`);

window.scrolled = () => document.querySelector("#scroller").scrollTop;
window.cursor = (line = LINES.drawn) => getComputedStyle(stageOf(line)).cursor;
window.overlay = (line = LINES.drawn) => {
  const controls = stageOf(line)?.querySelector(".diagram-controls");
  return {
    present: Boolean(controls),
    opacity: controls && Number(getComputedStyle(controls).opacity),
    level: controls?.querySelector(".diagram-level")?.textContent ?? "",
    stamped: controls?.querySelectorAll("[data-line], [data-line-end]").length ?? 0,
  };
};
window.press = (name, line = LINES.drawn) => {
  const control = stageOf(line).querySelector(`button[aria-label="${name}"]`);
  control.click();
  const drawn = stageOf(line).querySelector("svg");
  return {
    stepping: stageOf(line).classList.contains("stepping"),
    duration: getComputedStyle(drawn).transitionDuration,
  };
};
window.stamped = () => root.querySelectorAll("[data-line]").length;
window.focused = () => Boolean(document.activeElement?.closest?.(".diagram-controls"));
window.selected = () => String(window.getSelection() ?? "");
window.stageBox = (line = LINES.drawn) => {
  const box = stageOf(line).getBoundingClientRect();
  return { x: box.x, y: box.y, width: box.width, height: box.height };
};
// The transform read back off the element, so what is asserted is what paints.
window.view = (line = LINES.drawn) => {
  const stage = stageOf(line);
  const drawn = stage?.querySelector("svg");
  if (!drawn) return null;
  const matrix = new DOMMatrix(getComputedStyle(drawn).transform);
  return {
    scale: matrix.a,
    tx: matrix.e,
    ty: matrix.f,
    base: Math.min(stage.clientWidth / Number.parseFloat(drawn.style.width), 1),
  };
};
// Where in the diagram's own coordinates a point on the screen falls.
window.contentAt = (clientX, clientY, line = LINES.drawn) => {
  const seen = window.view(line);
  const box = window.stageBox(line);
  return { x: (clientX - box.x - seen.tx) / seen.scale, y: (clientY - box.y - seen.ty) / seen.scale };
};
window.settled = () =>
  new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));

window.probe = (line = LINES.drawn) => {
  const block = root.querySelector(`pre[data-line="${line}"]`);
  const drawn = block?.querySelector("svg");
  const stage = block?.querySelector(".diagram-stage");
  const note = block?.querySelector(".diagram-error");
  const box = drawn?.getBoundingClientRect();
  const bounds = block?.getBoundingClientRect();
  return {
    found: Boolean(block),
    line: block?.dataset.line,
    lineEnd: block?.dataset.lineEnd,
    pending: block?.classList.contains("diagram-pending"),
    failed: block?.classList.contains("diagram-failed"),
    opacity: block && Number(getComputedStyle(block).opacity),
    code: Boolean(block?.querySelector("code")),
    drawn: block?.classList.contains("diagram"),
    gutter: block && getComputedStyle(block, "::before").content,
    gutterLine: block && getComputedStyle(block, "::before").lineHeight,
    error: note?.textContent ?? "",
    notes: block?.querySelectorAll(".diagram-error").length ?? 0,
    markup: note?.children.length ?? 0,
    stated: note ? note.getBoundingClientRect().height : 0,
    width: box?.width ?? 0,
    height: box?.height ?? 0,
    tall: bounds?.height ?? 0,
    block: bounds?.width ?? 0,
    scrolls: (block?.scrollWidth ?? 0) > (block?.clientWidth ?? 0),
    zoomable: block?.classList.contains("zoomable"),
    controls: block?.querySelectorAll(".diagram-controls").length ?? 0,
    staged: Boolean(stage),
    position: block && getComputedStyle(block).position,
    styleWidth: drawn?.style.width ?? "",
    styleHeight: drawn?.style.height ?? "",
    transform: drawn && getComputedStyle(drawn).transform,
    boxWidth: block?.clientWidth ?? 0,
    boxHeight: block?.clientHeight ?? 0,
    slack:
      stage && drawn
        ? stage.getBoundingClientRect().right - drawn.getBoundingClientRect().right
        : 0,
    container: root.getBoundingClientRect().width,
    page: document.documentElement.scrollWidth,
    viewport: window.innerWidth,
  };
};
