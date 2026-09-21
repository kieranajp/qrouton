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


if (new URLSearchParams(location.search).has("links")) {
  const [{ links }, { mount }, { default: ContextMenu }] = await Promise.all([
    import("../src/lib/panes/actions.js"), import("svelte"), import("../src/lib/shell/ContextMenu.svelte"),
  ]);
  root.innerHTML = "";
  const calls = [];
  const actions = new Map();
  for (const id of ["vault-pane", "ordinary-pane"]) {
    const source = id === "vault-pane" ? "vault://shared/example/R1" : "notes/source.md";
    const rendered = render("# Heading With Spaces\n\n[Encoded](target.md#H%C3%A9llo%20%E4%B8%96%E7%95%8C) [Raw](target.md#Heading%20With%20Spaces) [Duplicate](target.md#heading-with-spaces-1)\n\n" + "padding paragraph\n\n".repeat(30) + "## Héllo 世界\n\nUnicode destination\n\n## Heading With Spaces\n\nDuplicate destination\n");
    const pane = document.createElement("article");
    pane.dataset.paneDocument = id; pane.dataset.documentSource = source;
    pane.style.cssText = "height:220px;overflow:auto;border:1px solid;padding:8px;margin-bottom:12px";
    pane.innerHTML = `<h1 id="${rendered.titleAnchor}">${rendered.title}</h1>${rendered.body}`;
    root.appendChild(pane);
    actions.set(id, { pane, source, action: links(pane, { id, source }) });
  }
  window.wailsCall = async (name, input) => {
    if (!name.endsWith(".OpenDocumentLink")) return;
    calls.push(input);
    const entry = actions.get(input.source);
    const fragment = decodeURIComponent(input.href.split("#")[1] ?? "");
    entry.action.update({id:input.source,source:entry.source,fragment,request:calls.length});
    return input.source;
  };
  window.navigationCalls = () => [...calls];
  mount(ContextMenu, { target: document.body });
}
