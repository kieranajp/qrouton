import "../src/tokens/scale.css";
import { mount } from "svelte";
import { emitWailsEvent } from "./wails-runtime.js";
import { STICKER_DUPLICATE, UI_SCALE_EVENT } from "../src/lib/bridge/generated.js";
import { startScale } from "../src/lib/scale.svelte.js";
import SettingsOverlay from "../src/lib/settings/SettingsOverlay.svelte";

// ?fail=<method> makes that one bridge call reject, which is how a workbench
// that cannot answer is reproduced without a workbench.
const params = new URLSearchParams(location.search);
const failing = params.get("fail") ?? "";
const invalid = params.get("invalid") ?? "";
const pinned = (params.get("pinned") ?? "").split(",");
const restart = params.get("restart") === "1";
const ollama = {
  state: params.get("ollama") ?? "ready",
  model: "nomic-embed-text",
  installed: params.get("installed") === "1",
};
const gh = params.get("gh") ?? "cli";
const ghCheck = params.get("check") ?? "valid";
const linear = {
  path: "~/.linear/coding-tools.json",
  snippet: '{\n  "openIssue": {\n    "path": "/usr/local/bin/qrouton",\n    "args": ["--linear-issue", "{{issue.identifier}}"]\n  }\n}',
  replaces: params.get("linear") === "replaces",
};
const saves = [];
const query = new URLSearchParams(location.search);
let closed = 0;
const runnerCalls = [];
const bridgeCalls = [];
const calls = [];
const runners = [
  { id: "codex", label: "Codex CLI", reviewer: { reasoningEditable: true, reasoningHelp: "Editable effort; blank uses the provider default.", modelHelp: "Editable model identifier." } },
  { id: "claude", label: "Claude Code", reviewer: { reasoningValues: ["low", "medium", "high", "xhigh", "max"], reasoningHelp: "Choose a supported effort or the provider default.", modelHelp: "Editable model identifier." } },
  { id: "opencode", label: "OpenCode", reviewer: { reasoningValues: [], reasoningHelp: "Only provider default reasoning is supported.", modelHelp: "Use provider/model." } },
];
const installed = () => runners.filter((runner) => !((query.has("noCodex") || query.has("unavailable")) && runner.id === "codex"));


window.settingsFixture = {
  saves: () => [...saves],
  closed: () => closed,
  closes: () => closed,
  runnerCalls: () => [...runnerCalls],
  bridgeCalls: () => [...bridgeCalls],
  calls: () => [...calls],
  announceScale: (percent) => emitWailsEvent(UI_SCALE_EVENT, percent),
};

window.wailsCall = async (name, input) => {
  bridgeCalls.push(name);
  calls.push(name.split(".").pop());
  if (failing && name.endsWith("." + failing)) throw new Error("config.json: permission denied");
  if (name.endsWith(".Runners")) {
    runnerCalls.push({ name, input });
    if (input) {
      const overrides = JSON.parse(input);
      return installed().filter((runner) => overrides[runner.id]?.[0] !== "missing");
    }
    return installed();
  }
  if (name.endsWith("Placement.Check")) window.placementChecks = (window.placementChecks ?? 0) + 1;
  if (failing && name.endsWith("." + failing)) throw new Error("config.json: permission denied");
  if (name.endsWith("Ollama.Check")) return ollama;
  if (name.endsWith(".GitHubStatus")) {
    const check = gh === "cli" || gh === "env" ? ghCheck : "";
    return { state: gh, check, login: check === "valid" ? "kieranajp" : "", variable: "GITHUB_TOKEN" };
  }
  if (name.endsWith(".Load"))
    return {
      reviewer: query.has("enabled") || query.has("unavailable") ? { harness: "codex", model: "saved-model", reasoning: "medium" } : null,
      reviewerError: query.has("unavailable") ? "codex is unavailable" : "",
      orgs: ["acme"],
      root: "/sessions",
      editor: "",
      launch: "",
      linear,
      stickers: [
        { id: "star", colour: "blue", symbol: "star", label: "Important" },
        { id: "bookmark", colour: "green", symbol: "bookmark", label: "Read later" },
        { id: "question", colour: "orange", symbol: "question", label: "Needs follow-up" },
        { id: "exclamation", colour: "red", symbol: "exclamation", label: "Has bugs" },
      ],
      chime: true,
      uiScale: 130,
      uiScaleSteps: [80, 90, 100, 110, 120, 130, 140, 150],
      thoughts: {
        default: "",
        derived: "/sessions/thoughts",
        roots: [
          { id: "work", path: "/sync/work", orgs: "acme" },
          { id: "club", path: "/sync/club", orgs: "club, club-labs", missing: true },
        ],
      },
      pinned: { root: pinned.includes("root"), orgs: pinned.includes("orgs") },
      runners: [
        { id: "claude", label: "Claude Code" },
        { id: "codex", label: "Codex" },
        { id: "opencode", label: "OpenCode" },
        { id: "agy", label: "Antigravity" },
      ],
    };
  if (name.endsWith(".InstallLinear")) return { ...linear, replaces: false };
  if (name.endsWith(".Save")) {
    saves.push(input);
    if (input?.reviewer?.harness === "opencode" && input.reviewer.reasoning) {
      throw new Error("reviewer: OpenCode supports only provider default reasoning");
    }
    if (invalid === "thoughts") throw new Error("thoughts: an org maps to two thoughts roots");
    const blank = (input?.stickers ?? []).findIndex((item) => !item.label.trim());
    if (invalid === "stickers" && blank >= 0) throw new Error(`stickers.${blank}: cannot be empty`);
    const pairs = (input?.stickers ?? []).map((item) => `${item.colour}/${item.symbol}`);
    const repeat = pairs.findIndex((pair, index) => pairs.indexOf(pair) !== index);
    if (repeat >= 0) throw new Error(`stickers.${repeat}: ${STICKER_DUPLICATE}`);
    return { restartRequired: restart };
  }
  return undefined;
};

startScale();
mount(SettingsOverlay, { target: document.querySelector("#fixture"), props: { onClose: () => closed++ } });
