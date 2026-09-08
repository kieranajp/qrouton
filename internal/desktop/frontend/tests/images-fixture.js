import "../src/tokens/index.css";
import { mount, unmount } from "svelte";
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
let windowID = "images-1";
let component;
let initialRelease;
let focusRelease;
window.deferSelection = false;
window.failSelection = false;
window.delayInitial = new URLSearchParams(location.search).has("delay");
window.bridgeCalls = [];
window.wailsCall = async (name, ...args) => {
  window.bridgeCalls.push({ name, args });
  if (name.endsWith(".Chrome.Snapshot")) return { activity: "idle" };
  if (name.endsWith(".Windows.Content")) {
    const snapshot = structuredClone(doc);
    if (window.delayInitial) return new Promise((resolve) => { initialRelease = () => resolve(snapshot); });
    return snapshot;
  }
  if (name.endsWith(".Windows.FocusImage")) {
    const [slug, id, index] = args;
    if (window.deferSelection) await new Promise((resolve) => { focusRelease = resolve; });
    if (window.failSelection || slug !== "fixture" || id !== windowID || index < 1 || index > doc.images.length) throw new Error("Selection refused");
    window.pushImages({ currentIndex: index, revision: doc.revision + 1 });
    return { currentIndex: index, count: doc.images.length, revision: doc.revision };
  }
};
window.pushImages = (changes) => {
  doc = { ...doc, ...changes };
  emitWailsEvent("window:content:" + windowID, doc);
};
window.errors = [];
addEventListener("error", (event) => window.errors.push(String(event.message)));
const attach = () => mount(DockedDocument, { target: document.querySelector("#fixture"), props: { id: windowID, slug: "fixture", active: true } });
component = attach();
window.releaseInitial = () => initialRelease?.();
window.replaceGallery = async (changes = {}) => {
  await unmount(component);
  windowID = "images-2";
  doc = { ...doc, images: [...doc.images].reverse().map((image, index) => ({ ...image, url: image.url + `${image.url.includes("?") ? "&" : "?"}replacement=${index}` })), currentIndex: 1, revision: 1, ...changes };
  window.delayInitial = false;
  component = attach();
};

window.pushOldImages = () => emitWailsEvent("window:content:images-1", { ...doc, revision: 100, currentIndex: 3 });

window.releaseFocus = () => focusRelease?.();
