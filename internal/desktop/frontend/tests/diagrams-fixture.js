import "../src/tokens/typography.css";
import "../src/lib/panes/markdown.css";
import { apply } from "../src/lib/panes/diagrams.js";
import { render } from "../src/lib/panes/markdown.js";

const FENCE_LINE = 3;
const root = document.querySelector("#markdown-root");

root.innerHTML = render("Before.\n\n```d2\ndirection: right\na -> b\n```\n\nAfter.\n").body;

// Shaped like d2's own output: an outer element carrying a viewBox and no size,
// wrapping one that carries both.
const svg =
  '<svg xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="xMinYMin meet" viewBox="0 0 1642 108">' +
  '<svg class="d2-svg" width="1642" height="108" viewBox="-21 -21 1642 108">' +
  '<rect x="-21" y="-21" width="1642" height="108" fill="#24273a"></rect>' +
  '<text x="40" y="60" fill="#cad3f5">a</text></svg></svg>';

window.pending = () => apply(root, [{ line: FENCE_LINE }]);
window.draw = () => apply(root, [{ line: FENCE_LINE, svg }]);
window.fail = () => apply(root, [{ line: FENCE_LINE, error: "diagram took too long to lay out" }]);
window.probe = () => {
  const block = root.querySelector(`pre[data-line="${FENCE_LINE}"]`);
  const drawn = block?.querySelector("svg");
  const box = drawn?.getBoundingClientRect();
  return {
    found: Boolean(block),
    line: block?.dataset.line,
    lineEnd: block?.dataset.lineEnd,
    pending: block?.classList.contains("diagram-pending"),
    opacity: block && Number(getComputedStyle(block).opacity),
    code: Boolean(block?.querySelector("code")),
    drawn: block?.classList.contains("diagram"),
    gutter: block && getComputedStyle(block, "::before").content,
    gutterLine: block && getComputedStyle(block, "::before").lineHeight,
    width: box?.width ?? 0,
    height: box?.height ?? 0,
    block: block?.getBoundingClientRect().width ?? 0,
    scrolls: (block?.scrollWidth ?? 0) > (block?.clientWidth ?? 0),
    container: root.getBoundingClientRect().width,
    page: document.documentElement.scrollWidth,
    viewport: window.innerWidth,
  };
};
