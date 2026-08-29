import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import TabsFixture from "./TabsFixture.svelte";

// One tab of each shape a session holds: a running shell, a badged plan, and a
// document with no id to badge with.
const TABS = [
  { id: "w1", label: "Shell", kind: "terminal", status: "running" },
  { id: "w2", label: "Pane smoke test", badge: "[P002]", kind: "document" },
  { id: "w3", label: "◆ Findings", kind: "document" },
];

// Narrow squeezes the plan tab into the overflow menu, which only ever holds
// tabs the reader has not selected.
const params = new URLSearchParams(location.search);
const narrow = params.has("narrow");
if (narrow) document.querySelector("#fixture").style.width = "230px";

window.wailsCall = async (name) =>
  name.endsWith(".Surfaces")
    ? { session: "fixture", selected: narrow ? "w1" : "w2", tabs: TABS }
    : undefined;

window.labels = () =>
  [...document.querySelectorAll(".tab")].map((tab) => ({
    text: tab.querySelector(".label").textContent,
    title: tab.getAttribute("title"),
    badge: tab.querySelector(".badge")?.textContent ?? "",
    badgeColour: tab.querySelector(".badge")
      ? getComputedStyle(tab.querySelector(".badge")).color
      : "",
  }));
window.menuLabels = () =>
  [...document.querySelectorAll(".menu .item .label")].map((item) =>
    item.textContent.replace(/\s+/g, " ").trim(),
  );

mount(TabsFixture, { target: document.querySelector("#fixture") });
