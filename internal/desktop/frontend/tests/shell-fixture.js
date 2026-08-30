import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import Session from "../src/Session.svelte";
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
  return undefined;
};

mount(Session, { target: document.querySelector("#fixture") });

window.shell = {
  chrome: (fields) => emitWailsEvent("chrome:update", { slug: "octopus", ...fields }),
  windows: (selected, tabs) =>
    emitWailsEvent("window:open", { session: "octopus", selected, tabs }),
  refuseSelect: (refuse) => (refuseSelect = refuse),
  selects: () =>
    calls.filter(({ name }) => name.endsWith("Windows.Select")).map(({ args }) => args),
  started: () =>
    calls
      .filter(({ name }) => name.endsWith(".Start"))
      .map(({ name, args }) => [name.split(".").at(-2), args[0]]),
};
