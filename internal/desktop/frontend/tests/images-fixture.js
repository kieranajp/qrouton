import "../src/tokens/index.css";
import { mount } from "svelte";
import DockedDocument from "../src/lib/DockedDocument.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

const asset = (file) => `/tests/fixtures/images/${file}`;
const entries = [
  { source: "thoughts/assets/z/screen.png", url: asset("small.png") + "?entry=1" },
  { source: "thoughts/assets/a/screen.png", url: asset("large.png") + "?entry=2" },
  { source: "thoughts/assets/z/screen.png", url: asset("small.png") + "?entry=3" },
  { source: "thoughts/assets/日本語 and spaces/" + "long-caption-".repeat(12) + ".jpg", url: asset("sample.jpg") },
  ...["gif", "webp", "avif"].map((ext) => ({ source: `thoughts/assets/sample.${ext}`, url: asset(`sample.${ext}`) })),
];
let doc = { text: "", source: "", format: "images", images: entries, currentIndex: 1, revision: 1, line: 0, to: 0 };
window.bridgeCalls = [];
window.wailsCall = async (name, ...args) => {
  window.bridgeCalls.push({ name, args });
  if (name.endsWith(".Chrome.Snapshot")) return { activity: "idle" };
  if (name.endsWith(".Windows.Content")) return doc;
};
window.pushImages = (changes) => {
  doc = { ...doc, ...changes };
  emitWailsEvent("window:content:images-1", doc);
};
window.errors = [];
addEventListener("error", (event) => window.errors.push(String(event.message)));
mount(DockedDocument, { target: document.querySelector("#fixture"), props: { id: "images-1", active: true } });
