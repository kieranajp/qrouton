import "./share.css";
import { artifactTone } from "../lib/artifacts.js";
import { apply as applyDiagrams } from "../lib/panes/diagrams.js";
import { render } from "../lib/panes/markdown.js";

// Both payloads arrive base64-encoded so nothing in them can close the script
// tag carrying it.
function decode(id) {
  const node = document.getElementById(id);
  if (!node) return "";
  const bytes = Uint8Array.from(atob(node.textContent.trim()), (c) => c.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

function payload() {
  const text = decode("qrouton-document");
  if (!text) return { kind: "NOTE", source: "", markdown: "" };
  const kindEnd = text.indexOf("\n");
  const sourceEnd = text.indexOf("\n", kindEnd + 1);
  return {
    kind: text.slice(0, kindEnd),
    source: text.slice(kindEnd + 1, sourceEnd),
    markdown: text.slice(sourceEnd + 1),
  };
}

// The cube is drawn rather than imported: CubeMark is a Svelte component, and a
// shared page has no runtime to mount it into.
function cube(size, tone) {
  const off = Math.round(size * 0.18);
  const inner = size - off;
  const mark = document.createElement("span");
  mark.className = "mark";
  mark.style.width = mark.style.height = `${size}px`;
  for (const face of ["back", "face"]) {
    const square = document.createElement("span");
    square.className = `square ${face}`;
    square.style.width = square.style.height = `${inner}px`;
    if (face === "back") square.style.left = square.style.top = `${off}px`;
    if (face === "face") square.style.background = tone;
    mark.append(square);
  }
  return mark;
}

const { kind, source, markdown } = payload();
const { title, body } = render(markdown);
const heading = title || source.split("/").pop();

const article = document.createElement("article");
article.className = "document";

if (source) {
  const path = document.createElement("p");
  path.className = "caps dim";
  path.textContent = source;
  article.append(path);
}

if (heading) {
  const row = document.createElement("div");
  row.className = "title";
  const label = document.createElement("span");
  label.textContent = heading;
  row.append(cube(18, artifactTone(kind)), label);
  article.append(row);
}

const prose = document.createElement("div");
prose.className = "markdown";
prose.innerHTML = body;
article.append(prose);

document.body.append(article);

// The diagrams were laid out before the page was written, so every fence is
// already settled: none of them can be left waiting.
const drawings = decode("qrouton-diagrams");
if (drawings) applyDiagrams(prose, JSON.parse(drawings));
