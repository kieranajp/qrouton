import { mount, unmount } from "svelte";
import Overlay from "../src/lib/assembly/Overlay.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

const calls = [];
let resolveBegin;

window.wailsCall = (name, ...args) => {
  calls.push({ name, args });
  if (name.endsWith(".Begin")) return new Promise((resolve) => (resolveBegin = resolve));
  if (name.endsWith("Orgs.List")) return [...window.orgList];
  if (name.endsWith(".Prefixes") || name.endsWith(".Runners") || name.endsWith(".List") || name.endsWith(".Cached") || name.endsWith(".Check")) return [];
  if (name.endsWith(".Preview")) return "";
  return undefined;
};

window.orgList = ["acme"];

const component = mount(Overlay, {
  target: document.querySelector("#fixture"),
  props: { onClose: () => {} },
});

window.assembly = {
  visible: () => !!document.querySelector(".dialog"),
  resolveBegin: (seed) => resolveBegin(seed),
  calls: () => [...calls],
  emit: (name, data) => emitWailsEvent(name, data),
  close: () => unmount(component),
};
