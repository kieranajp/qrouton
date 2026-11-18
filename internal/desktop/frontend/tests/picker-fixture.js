import "../src/tokens/scale.css";
import { mount } from "svelte";
import PickerOverlay from "../src/lib/assembly/PickerOverlay.svelte";
import { ASSEMBLY_PROGRESS_EVENT, PICKER_ABORT, PICKER_CONFIRM } from "../src/lib/bridge/generated.js";
import { emitWailsEvent } from "./wails-runtime.js";

const calls = [];
let adding;
let abortFinds = true;
let closeRequests = 0;

window.wailsCall = (name, ...args) => {
  calls.push({ name, args });
  if (name === PICKER_ABORT) return abortFinds;
  if (name === PICKER_CONFIRM) return new Promise((resolve, reject) => (adding = { resolve, reject }));
  if (name.endsWith("Picker.Load")) return { branch: "fix/octopus-4b2a", repos: [] };
  if (name.endsWith("Orgs.List")) return ["acme"];
  if (name.endsWith("Repositories.Cached")) return [{ org: "acme", name: "api", default_branch: "main" }];
  return undefined;
};

mount(PickerOverlay, {
  target: document.querySelector("#fixture"),
  props: { slug: "octopus", onClose: () => closeRequests++ },
});

window.picker = {
  progress: (event) => emitWailsEvent(ASSEMBLY_PROGRESS_EVENT, event),
  refuse: (reason) => adding.reject(new Error(reason)),
  confirms: () => calls.filter(({ name }) => name === PICKER_CONFIRM).length,
  aborts: () => calls.filter(({ name }) => name === PICKER_ABORT).map(({ args }) => args),
  closeRequests: () => closeRequests,
  abortFinds: (running) => (abortFinds = running),
};
