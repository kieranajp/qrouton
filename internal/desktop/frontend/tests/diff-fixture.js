import { mount, unmount } from "svelte";
import DiffPane from "../src/lib/panes/DiffPane.svelte";

const root = document.querySelector("#diff-root");

window.defaultDiff = [
  "=== app ===",
  "diff --git a/example.txt b/example.txt",
  "index 1111111..2222222 100644",
  "--- a/example.txt",
  "+++ b/example.txt",
  "Binary files a/image.png and b/image.png differ",
  "@@ -98,4 +198,5 @@ section",
  " context before",
  "--- metadata-looking deletion",
  "+++ metadata-looking addition",
  "-old line",
  "+new line",
  ` ${"long-token-".repeat(30)}`,
  "+extra addition",
  "\\ No newline at end of file",
  "",
].join("\n");

let component;

async function render(text) {
  if (component) await unmount(component);
  component = mount(DiffPane, {
    target: root,
    props: { doc: { text, format: "diff", source: "" } },
  });
}

window.setDiff = render;
window.setPaneWidth = (pixels) => {
  root.style.width = `${pixels}px`;
};
window.selectDiff = () => {
  const range = document.createRange();
  range.selectNodeContents(root.querySelector(".diff-grid"));
  const selection = window.getSelection();
  selection.removeAllRanges();
  selection.addRange(range);
  return selection.toString();
};

await render(window.defaultDiff);
