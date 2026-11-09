import "../src/tokens/scale.css";
import "../src/tokens/typography.css";
import "../src/lib/panes/markdown.css";
import { render } from "../src/lib/panes/markdown.js";

const sentence = "For each of `legacy-decides`, `verifier-decides`, and `verifier-only`: dispatch `auth-middleware-edge-publish` → note `version` in the version → bump.";
const root = document.querySelector("#markdown-root");

root.innerHTML = render(`${sentence}\n\n- [ ] ${sentence}\n`).body;

window.taskLayout = () => {
  const paragraph = root.querySelector(":scope > p");
  const item = root.querySelector(".task-list-item");
  return {
    display: getComputedStyle(item).display,
    paragraph: paragraph.getBoundingClientRect().height,
    task: item.getBoundingClientRect().height,
    line: Number.parseFloat(getComputedStyle(item).lineHeight),
    ...lineStarts(item),
    codeBreak: getComputedStyle(item.querySelector("code")).boxDecorationBreak,
  };
};

/** The left edge of the item's first and last line of text, checkbox excluded. */
function lineStarts(item) {
  const range = document.createRange();
  range.setStartAfter(item.querySelector("input"));
  range.setEndAfter(item.lastChild);
  const rects = [...range.getClientRects()].filter((rect) => rect.width > 0);
  const top = Math.min(...rects.map((rect) => rect.top));
  const bottom = Math.max(...rects.map((rect) => rect.top));
  const startOf = (row) => Math.min(...rects.filter((rect) => Math.abs(rect.top - row) < 2).map((rect) => rect.left));
  return { firstStart: startOf(top), lastStart: startOf(bottom), lines: new Set(rects.map((rect) => Math.round(rect.top))).size };
}
