import "../src/tokens/scale.css";
import { mount } from "svelte";
import { emitWailsEvent } from "./wails-runtime.js";
import { UI_SCALE_EVENT } from "../src/lib/bridge/generated.js";
import SettingsOverlay from "../src/lib/settings/SettingsOverlay.svelte";

// ?fail=<method> makes that one bridge call reject, which is how a workbench
// that cannot answer is reproduced without a workbench.
const failing = new URLSearchParams(location.search).get("fail") ?? "";
const invalid = new URLSearchParams(location.search).get("invalid") ?? "";
const saves = [];
const query = new URLSearchParams(location.search);
let closed = 0;
const runnerCalls = [];
const bridgeCalls = [];
const runners = [
  { id: "codex", label: "Codex CLI", reviewer: { reasoningEditable: true, reasoningHelp: "Editable effort; blank uses the provider default.", modelHelp: "Editable model identifier." } },
  { id: "claude", label: "Claude Code", reviewer: { reasoningValues: ["low", "medium", "high", "xhigh", "max"], reasoningHelp: "Choose a supported effort or the provider default.", modelHelp: "Editable model identifier." } },
  { id: "opencode", label: "OpenCode", reviewer: { reasoningValues: [], reasoningHelp: "Only provider default reasoning is supported.", modelHelp: "Use provider/model." } },
];
const installed = () => runners.filter((runner) => !((query.has("noCodex") || query.has("unavailable")) && runner.id === "codex"));


window.settingsFixture = {
  saves: () => [...saves],
  closed: () => closed,
  runnerCalls: () => [...runnerCalls],
  bridgeCalls: () => [...bridgeCalls],
  announceScale: (percent) => emitWailsEvent(UI_SCALE_EVENT, percent),
};

window.wailsCall = async (name, input) => {
  bridgeCalls.push(name);
  if (failing && name.endsWith("." + failing)) throw new Error("config.json: permission denied");
  if (name.endsWith(".Runners")) {
    runnerCalls.push({ name, input });
    if (input) {
      const overrides = JSON.parse(input);
      return installed().filter((runner) => overrides[runner.id]?.[0] !== "missing");
    }
    return installed();
  }
  if (name.endsWith(".Load"))
    return {
      reviewer: query.has("enabled") || query.has("unavailable") ? { harness: "codex", model: "saved-model", reasoning: "medium" } : null,
      reviewerError: query.has("unavailable") ? "codex is unavailable" : "",
      orgs: ["acme"],
      root: "/sessions",
      editor: "",
      launch: "",
      linear: "",
      stickerLabels: {
        star: "Important",
        bookmark: "Read later",
        question: "Needs follow-up",
        exclamation: "Has bugs",
      },
      chime: true,
      uiScale: 130,
      uiScaleSteps: [80, 90, 100, 110, 120, 130, 140, 150],
      thoughts: {
        default: "",
        derived: "/sessions/thoughts",
        roots: [
          { id: "work", path: "/sync/work", orgs: "acme" },
          { id: "club", path: "/sync/club", orgs: "club, club-labs" },
        ],
      },
    };
  if (name.endsWith(".Save")) {
    saves.push(input);
    if (input?.reviewer?.harness === "opencode" && input.reviewer.reasoning) {
      throw new Error("reviewer: OpenCode supports only provider default reasoning");
    }
    if (invalid === "thoughts") throw new Error("thoughts: an org maps to two thoughts roots");
    if (invalid && !input?.stickerLabels?.[invalid]?.trim()) {
      throw new Error(`${invalid}: cannot be empty`);
    }
    return { restartRequired: false };
  }
  return undefined;
};

mount(SettingsOverlay, { target: document.querySelector("#fixture"), props: { onClose: () => { closed++; } } });
