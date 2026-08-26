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
  };
};
