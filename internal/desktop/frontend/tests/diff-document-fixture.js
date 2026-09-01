import "../src/tokens/index.css";
import { mount } from "svelte";
import DockedDocument from "../src/lib/DockedDocument.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

export const DEFAULT_DIFF = [
  "diff --git a/notes.txt b/notes.txt",
  "index 1111111..2222222 100644",
  "--- a/notes.txt",
  "+++ b/notes.txt",
  "@@ -1 +1 @@",
  "-old note",
  "+new note",
  "diff --git a/other.txt b/other.txt",
  "index 3333333..4444444 100644",
  "--- a/other.txt",
  "+++ b/other.txt",
  "@@ -1 +1 @@",
  "-old other",
  "+new other",
].join("\n");

let rawText = DEFAULT_DIFF;
const documentFor = (text) => ({
  text,
  format: "diff",
  source: "thoughts/shared/diffs/D001-fixture.diff",
  path: "/sessions/fixture/thoughts/shared/diffs/D001-fixture.diff",
  kind: "",
  line: 0,
  to: 0,
  viewportEpoch: 1,
});

window.bridgeCalls = [];
window.wailsCall = async (name, ...args) => {
  window.bridgeCalls.push({ name, args });
  if (name.endsWith(".Chrome.Snapshot")) {
    return { activity: "idle", sessions: [], documents: [], repositoryDocuments: [], repos: [] };
  }
  if (name.endsWith(".Windows.Content")) return documentFor(rawText);
  return undefined;
};

window.setDiff = (text) => {
  rawText = String(text);
  emitWailsEvent("window:content:document-1", documentFor(rawText));
};

window.largeDiff = (count = 220) =>
  Array.from({ length: count }, (_, index) => [
    `diff --git a/file-${index}.txt b/file-${index}.txt`,
    "index 1111111..2222222 100644",
    `--- a/file-${index}.txt`,
    `+++ b/file-${index}.txt`,
    "@@ -1,50 +1,50 @@",
    ...Array.from({ length: 25 }, (_, line) => ` common-${index}-${line}`),
    `-needle-${index}`,
    `+needle-${index}`,
    ...Array.from({ length: 24 }, (_, line) => ` common-${index}-${line + 25}`),
  ].join("\n")).join("\n");

window.errors = [];
addEventListener("error", (event) => window.errors.push(String(event.message)));
mount(DockedDocument, { target: document.querySelector("#fixture"), props: { id: "document-1", active: true } });
