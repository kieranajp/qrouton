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
];
const LINES = { drawn: 3, broken: 10, embedded: 16 };
const TIMEOUT = "diagram took too long to lay out";
const root = document.querySelector("#markdown-root");

root.innerHTML = render(DOCUMENT.join("\n")).body;

// Shaped like d2's own output: an outer element carrying a viewBox and no size,
// wrapping one that carries both.
const svg =
  '<svg xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="xMinYMin meet" viewBox="0 0 1642 108">' +
  '<svg class="d2-svg" width="1642" height="108" viewBox="-21 -21 1642 108">' +
  '<rect x="-21" y="-21" width="1642" height="108" fill="#24273a"></rect>' +
  '<text x="40" y="60" fill="#cad3f5">a</text></svg></svg>';

window.lines = LINES;
window.pending = () => apply(root, Object.values(LINES).map((line) => ({ line })));
window.draw = () => apply(root, [{ line: LINES.drawn, svg }]);
window.fail = (line = LINES.drawn, error = TIMEOUT) => apply(root, [{ line, error }]);
window.settle = () =>
  apply(root, [
    { line: LINES.drawn, svg },
    { line: LINES.broken, error: "11:5: unexpected end of file" },
    { line: LINES.embedded, error: "diagram embeds HTML; |md| blocks are not rendered" },
  ]);
window.probe = (line = LINES.drawn) => {
  const block = root.querySelector(`pre[data-line="${line}"]`);
  const drawn = block?.querySelector("svg");
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
    container: root.getBoundingClientRect().width,
    page: document.documentElement.scrollWidth,
    viewport: window.innerWidth,
  };
};
