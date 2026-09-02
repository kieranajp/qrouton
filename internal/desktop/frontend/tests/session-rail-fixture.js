import "../src/tokens/index.css";
import { mount } from "svelte";
import SessionRailFixture from "./SessionRailFixture.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

const order = ["star", "bookmark", "question", "exclamation", ""];
const stickers = new Map();
const calls = [];
let delay = 40;
let failing = false;

window.sessionRailBridge = {
  calls: () => [...calls],
  failNext: () => (failing = true),
  setDelay: (milliseconds) => (delay = milliseconds),
  saveStickerLabels: (stickerLabels) => emitWailsEvent("chrome:update", { stickerLabels }),
};

window.wailsCall = async (name, slug) => {
  if (name.endsWith(".Show")) {
    calls.push({ method: "show", slug });
    window.sessionRail?.shown(slug);
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
