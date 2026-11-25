import "../src/tokens/scale.css";
import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import { WINDOW_CONTENT_EVENT } from "../src/lib/bridge/generated.js";
import TodoFixture from "./TodoFixture.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

export const TODO = [
  "---",
  "kind: todo",
  "---",
  "",
  "# Rollout",
  "",
  "Everything between the branch and production.",
  "",
  "## Before merge",
  "",
  "- [x] Merge acme/api#12 — live as v2.94.0",
  "  - Where: https://github.com/acme/api/pull/12",
  "- [ ] Apply the migration",
  "  - Where: enclave · production",
  "  - Check: `select count(*) from tenants`",
  "  - Run: `make migrate ENV=production`",
  "  This can run alongside the items above.",
  "- [-] Roll back the cache — not needed after all",
  "",
  "## Ship",
  "",
  "- [ ] Announce in the channel",
  "  - Where: acme/web#7",
  "  - Check: the post has a link",
  "",
].join("\n");

const hashOf = (text) => {
  let h = 0;
  for (const c of text) h = (Math.imul(h, 31) + c.charCodeAt(0)) | 0;
  return `h${(h >>> 0).toString(16)}`;
};

window.file = TODO;
let stale = null;

const document_ = () => ({
  text: window.file,
  hash: hashOf(window.file),
  format: "markdown",
  source: "thoughts/shared/todos/T1-fixture.md",
  path: "/sessions/fixture/thoughts/shared/todos/T1-fixture.md",
  kind: "TODO",
  line: 0,
  to: 0,
  viewportEpoch: 1,
});

const DASHBOARD = {
  viewer: "octocat",
  checkedAt: "2026-11-24T10:00:00Z",
  sessions: {
    fixture: {
      pullRequests: [{ repo: "acme/api", number: 12, title: "Add the tenants table", url: "https://github.com/acme/api/pull/12", state: "merged", author: "octocat" }],
    },
  },
};

window.calls = [];
window.wailsCall = async (name, ...args) => {
  window.calls.push({ name, args });
  if (name.endsWith(".Content")) return document_();
  if (name.endsWith(".RenderDiagrams")) return [];
  if (name.endsWith("Dashboard.Snapshot")) return DASHBOARD;
  if (name.endsWith(".SaveAnswers")) {
    const [, hash, text] = args;
    if (hold) await hold;
    if (stale !== null) {
      window.file = stale;
      stale = null;
    }
    if (hash !== hashOf(window.file)) throw new Error("the document changed on disk since the pane read it");
    window.file = text;
    return hashOf(text);
  }
  return undefined;
};

// The next save finds the file already rewritten to this text.
window.staleOnce = (text) => (stale = text);
let hold = null;
window.holdSaves = () => {
  hold = new Promise((resolve) => (window.releaseSaves = () => {
    hold = null;
    resolve();
  }));
};
window.saves = () => window.calls.filter((call) => call.name.endsWith(".SaveAnswers")).map((call) => call.args);
window.sends = () => window.calls.filter((call) => call.name.endsWith(".SendTodoUpdate")).map((call) => call.args);
window.TODO = TODO;
// What the workbench pushes when an item elsewhere is opened at its line.
window.focusLine = (line, focus) => emitWailsEvent(WINDOW_CONTENT_EVENT + "w1", { ...document_(), line, to: 0, focus });
window.pushContent = (text) => {
  window.file = text;
  emitWailsEvent(WINDOW_CONTENT_EVENT + "w1", document_());
};

mount(TodoFixture, { target: document.querySelector("#fixture") });
