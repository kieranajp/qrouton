import "../src/tokens/scale.css";
import { mount, unmount } from "svelte";
import Overlay from "../src/lib/assembly/Overlay.svelte";
import { PROFILES_STATUS } from "../src/lib/bridge/generated.js";
import { reactiveProps } from "./reactive-props.svelte.js";
import { emitWailsEvent } from "./wails-runtime.js";

const calls = [];
const settingsRequests = [];
let resolveBegin;
let resolveBranches;
let closeRequests = 0;

const row = (id, label, profile = "", profileLabel = "") => ({ id, label, profile, profileLabel, installed: true });
const DEFAULT_RUNNERS = [
  row("claude", "Claude Code"),
  row("codex", "Codex"),
  row("opencode", "OpenCode"),
  row("agy", "Antigravity"),
];

window.wailsCall = (name, ...args) => {
  calls.push({ name, args });
  if (name.endsWith(".Begin")) return new Promise((resolve) => (resolveBegin = resolve));
  if (name.endsWith("Orgs.List")) return [...window.orgList];
  if (name.endsWith(".Cached")) return [...window.repoList];
  if (name.endsWith("Repositories.Branches")) return new Promise((resolve) => (resolveBranches = resolve));
  if (name.endsWith(".Runners")) return structuredClone(window.runnerRows ?? DEFAULT_RUNNERS);
  if (name === PROFILES_STATUS) return structuredClone(window.profileStatuses ?? []);
  if (name.endsWith(".Prefixes") || name.endsWith(".List") || name.endsWith(".Check")) return [];
  if (name.endsWith(".Preview")) return window.previewBranch ?? "";
  return undefined;
};

window.orgList = ["acme"];
window.repoList = [{ org: "acme", name: "api", default_branch: "main", pushed_at: "2026-02-01T00:00:00Z" }];

const props = reactiveProps({
  onClose: () => closeRequests++,
  settingsOpen: false,
  onSettings: (tab) => settingsRequests.push(tab),
});
const component = mount(Overlay, { target: document.querySelector("#fixture"), props });

window.assembly = {
  visible: () => !!document.querySelector(".dialog"),
  resolveBegin: (seed) => resolveBegin(seed),
  resolveBranches: (answer) => resolveBranches(answer),
  calls: () => [...calls],
  closeRequests: () => closeRequests,
  settingsRequests: () => [...settingsRequests],
  setSettingsOpen: (open) => (props.settingsOpen = open),
  emit: (name, data) => emitWailsEvent(name, data),
  close: () => unmount(component),
};
