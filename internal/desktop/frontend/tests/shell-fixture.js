import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import Session from "../src/Session.svelte";
import { encode, terminalAt } from "../src/lib/xterm.js";
import { emitWailsEvent } from "./wails-runtime.js";

const calls = [];
let refuseSelect = false;

window.wailsCall = (name, ...args) => {
  calls.push({ name, args });
  if (name.endsWith("Windows.Select") && refuseSelect) {
    return Promise.reject(new Error("no such window"));
  }
  if (name.endsWith("Windows.Surfaces")) return { session: "octopus", selected: "", tabs: [] };
  if (name.endsWith("Windows.OpenShell")) return "window-9";
  if (name.endsWith("Picker.Load")) return { branch: "fix/octopus-4b2a", repos: [] };
  if (name.endsWith("Orgs.List") || name.endsWith("Repositories.Cached")) return [];
  return undefined;
};

mount(Session, { target: document.querySelector("#fixture") });

// What paints over what, at the one point the titlebar's menu and the pane
// chrome below it both cover. A pane header and a tab strip each carry a
// stacking order of their own, so the titlebar has to outrank the panes.
window.overlapOwner = () => {
  const menu = document.querySelector(".menu").getBoundingClientRect();
  const chrome = document.querySelector(".header, .strip").getBoundingClientRect();
  if (menu.bottom <= chrome.top) return "no overlap";
  const y = (Math.max(menu.top, chrome.top) + Math.min(menu.bottom, chrome.bottom)) / 2;
  const hit = document.elementFromPoint(menu.left + menu.width / 2, y);
  if (hit?.closest(".menu")) return "menu";
  return hit?.closest(".header, .strip") ? "pane chrome" : (hit?.tagName ?? "nothing");
};

window.shell = {
  chrome: (fields) => emitWailsEvent("chrome:update", { slug: "octopus", ...fields }),
  windows: (selected, tabs) =>
    emitWailsEvent("window:open", { session: "octopus", selected, tabs }),
  terminalData: (id, text, replay = false) =>
    emitWailsEvent("window:data:" + id, { encoded: encode(text), replay }),
  terminalText: () => {
    const term = terminalAt(document.querySelector(".human .host"));
    if (!term) return "";
    const lines = [];
    for (let i = 0; i < term.buffer.active.length; i++) {
      lines.push(term.buffer.active.getLine(i)?.translateToString(true) ?? "");
    }
    return lines.join("\n");
  },
  refuseSelect: (refuse) => (refuseSelect = refuse),
  selects: () =>
    calls.filter(({ name }) => name.endsWith("Windows.Select")).map(({ args }) => args),
  escalations: () =>
    calls.filter(({ name }) => name.endsWith("Picker.Escalate")).map(({ args }) => args),
  started: () =>
    calls
      .filter(({ name }) => name.endsWith(".Start"))
      .map(({ name, args }) => [name.split(".").at(-2), args[0]]),
};
