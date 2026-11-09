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

/** Where the item's first visible glyph and its last line start; the space after the checkbox is not a glyph. */
function lineStarts(item) {
  const text = item.querySelector("input").nextSibling;
  const glyph = text.data.search(/\S/);
  const first = document.createRange();
  first.setStart(text, glyph);
  first.setEnd(text, glyph + 1);
  const range = document.createRange();
  range.setStart(text, glyph);
  range.setEndAfter(item.lastChild);
  const rects = [...range.getClientRects()].filter((rect) => rect.width > 0);
  const bottom = Math.max(...rects.map((rect) => rect.top));
  const lastStart = Math.min(...rects.filter((rect) => Math.abs(rect.top - bottom) < 2).map((rect) => rect.left));
  return { firstStart: first.getBoundingClientRect().left, lastStart, lines: new Set(rects.map((rect) => Math.round(rect.top))).size };
}
