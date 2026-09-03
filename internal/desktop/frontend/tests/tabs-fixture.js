import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import { emitWailsEvent } from "./wails-runtime.js";
import TabsFixture from "./TabsFixture.svelte";

// One tab of each shape a session holds: a running shell, a badged plan, a
// badged research document that must not borrow the plan's colour, and a
// document with no id to badge with.
let tabs = [
  { id: "w1", label: "Shell", kind: "terminal", status: "running" },
  { id: "w2", label: "Pane smoke test", badge: "P002", artifact: "PLAN", kind: "document" },
  { id: "w3", label: "Pane selection", badge: "R001", artifact: "RESEARCH", kind: "document" },
  { id: "w4", label: "◆ Findings", kind: "document", status: "succeeded" },
];

// The strip draws the process state Go names, so a run that finished carries
// the same word here that the window registry sent.
window.dots = () =>
  [...document.querySelectorAll(".tab")].map((tab) => {
    const dot = tab.querySelector(".dot");
    return dot ? getComputedStyle(dot).backgroundColor : "";
  });
window.menuDots = () =>
  [...document.querySelectorAll(".menu .item")].map((item) => {
    const dot = item.querySelector(".dot");
    return dot ? getComputedStyle(dot).backgroundColor : "";
  });

// Narrow squeezes the plan tab into the overflow menu, which only ever holds
// tabs the reader has not selected.
const params = new URLSearchParams(location.search);
const narrow = params.has("narrow");
if (narrow) document.querySelector("#fixture").style.width = "230px";

const payload = () => ({ session: "fixture", selected: narrow ? "w1" : "w2", tabs });

// The fixture stands in for the window registry, so a reorder or a close only
// reaches the strip by coming back as the surfaces payload Go would send.
window.wailsCall = async (name, ...args) => {
  if (name.endsWith(".Surfaces")) return payload();
  if (name.endsWith(".Reorder")) {
    const [, id, to] = args;
    const moved = tabs.find((tab) => tab.id === id);
    tabs = tabs.filter((tab) => tab !== moved);
    tabs.splice(to, 0, moved);
    emitWailsEvent("window:open", payload());
  }
  if (name.endsWith(".Close")) {
    tabs = tabs.filter((tab) => tab.id !== args[0]);
    emitWailsEvent("window:open", payload());
  }
  return undefined;
};

window.labels = () =>
  [...document.querySelectorAll(".tab")].map((tab) => ({
    text: tab.querySelector(".label").textContent,
    title: tab.getAttribute("title"),
    badge: tab.querySelector(".tag")?.textContent ?? "",
    badgeColour: tab.querySelector(".tag")
      ? getComputedStyle(tab.querySelector(".tag")).backgroundColor
      : "",
  }));
window.menuLabels = () =>
  [...document.querySelectorAll(".menu .item .label")].map((item) =>
    item.textContent.replace(/\s+/g, " ").trim(),
  );

mount(TabsFixture, { target: document.querySelector("#fixture") });
