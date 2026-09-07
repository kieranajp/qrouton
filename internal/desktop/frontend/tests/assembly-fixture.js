import { mount, unmount } from "svelte";
import Overlay from "../src/lib/assembly/Overlay.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

const calls = [];
let resolveBegin;
let resolveBranches;

window.wailsCall = (name, ...args) => {
  calls.push({ name, args });
  if (name.endsWith(".Begin")) return new Promise((resolve) => (resolveBegin = resolve));
  if (name.endsWith("Orgs.List")) return [...window.orgList];
  if (name.endsWith(".Cached")) return [...window.repoList];
  if (name.endsWith("Repositories.Branches")) return new Promise((resolve) => (resolveBranches = resolve));
  if (name.endsWith(".Prefixes") || name.endsWith(".Runners") || name.endsWith(".List") || name.endsWith(".Check")) return [];
  if (name.endsWith(".Preview")) return "";
  return undefined;
};

window.orgList = ["acme"];
window.repoList = [{ org: "acme", name: "api", default_branch: "main", pushed_at: "2026-02-01T00:00:00Z" }];

const component = mount(Overlay, {
  target: document.querySelector("#fixture"),
  props: { onClose: () => {} },
});

window.assembly = {
  visible: () => !!document.querySelector(".dialog"),
  resolveBegin: (seed) => resolveBegin(seed),
  resolveBranches: (answer) => resolveBranches(answer),
  calls: () => [...calls],
  emit: (name, data) => emitWailsEvent(name, data),
  close: () => unmount(component),
};
