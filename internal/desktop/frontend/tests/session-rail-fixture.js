import "../src/tokens/index.css";
import { mount } from "svelte";
import SessionRailFixture from "./SessionRailFixture.svelte";
import { labelled } from "./session-rail-stickers.js";
import { emitWailsEvent } from "./wails-runtime.js";

const order = ["star", "bookmark", "question", "exclamation", ""];
const stickers = new Map();
const calls = [];
let delay = 40;
let failing = false;
let unloadFailing = false;
let reorderFailing = false;
let deferReorder = false;
let completeReorder;

window.sessionRailBridge = {
  calls: () => [...calls],
  failNext: () => (failing = true),
  failNextUnload: () => (unloadFailing = true),
  failNextReorder: () => (reorderFailing = true),
  deferReorder: () => (deferReorder = true),
  completeReorder: () => completeReorder?.(),
  setDelay: (milliseconds) => (delay = milliseconds),
  saveStickers: (list) => emitWailsEvent("chrome:update", { stickers: list }),
  saveStickerLabels: (labels) => emitWailsEvent("chrome:update", { stickers: labelled(labels) }),
};

window.wailsCall = async (name, slug, targetSlug) => {
  if (name.endsWith(".Show")) {
    calls.push({ method: "show", slug });
    window.sessionRail?.shown(slug);
    return;
  }
  if (name.endsWith(".Reorder")) {
    calls.push({ method: "reorder", slug, targetSlug });
    if (deferReorder) {
      await new Promise((resolve) => (completeReorder = resolve));
      deferReorder = false;
      completeReorder = undefined;
    }
    if (reorderFailing) {
      reorderFailing = false;
      throw new Error("row unavailable");
    }
    window.sessionRail?.reordered(slug, targetSlug);
    return;
  }
  if (name.endsWith(".Unload")) {
    calls.push({ method: "unload", slug });
    await new Promise((resolve) => setTimeout(resolve, delay));
    if (unloadFailing) {
      unloadFailing = false;
      throw new Error("session unavailable");
    }
    window.sessionRail?.unloaded(slug);
    return;
  }
  if (name.endsWith(".CycleSticker")) {
    calls.push({ method: "cycle", slug });
    await new Promise((resolve) => setTimeout(resolve, delay));
    if (failing) {
      failing = false;
      throw new Error("manifest unavailable");
    }
    const current = stickers.get(slug) ?? "";
    const next = order[(order.indexOf(current) + 1) % order.length];
    stickers.set(slug, next);
    window.sessionRail?.stickerChanged(slug, next);
    return next;
  }
  calls.push({ method: name.split(".").at(-1).toLowerCase(), slug });
};
mount(SessionRailFixture, { target: document.querySelector("#fixture") });
