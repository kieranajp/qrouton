import "../src/tokens/scale.css";
import { mount } from "svelte";
import { emitWailsEvent } from "./wails-runtime.js";
import { UI_SCALE_EVENT } from "../src/lib/bridge/generated.js";
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
const saves = [];
const calls = [];

window.settingsFixture = {
  saves: () => [...saves],
  calls: () => [...calls],
  announceScale: (percent) => emitWailsEvent(UI_SCALE_EVENT, percent),
};

window.wailsCall = async (name, input) => {
  calls.push(name.split(".").pop());
  if (failing && name.endsWith("." + failing)) throw new Error("config.json: permission denied");
  if (name.endsWith("Ollama.Check")) return ollama;
  if (name.endsWith(".GitHubStatus"))
    return { state: gh, login: gh === "cli" || gh === "env" ? "kieranajp" : "", variable: "GITHUB_TOKEN" };
  if (name.endsWith(".Load"))
    return {
      orgs: ["acme"],
      root: "/sessions",
      editor: "",
      launch: "",
      linear: "",
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
  if (name.endsWith(".Save")) {
    saves.push(input);
    if (invalid === "thoughts") throw new Error("thoughts: an org maps to two thoughts roots");
    const blank = (input?.stickers ?? []).findIndex((item) => !item.label.trim());
    if (invalid === "stickers" && blank >= 0) throw new Error(`stickers.${blank}: cannot be empty`);
    const pairs = (input?.stickers ?? []).map((item) => `${item.colour}/${item.symbol}`);
    const repeat = pairs.findIndex((pair, index) => pairs.indexOf(pair) !== index);
    if (repeat >= 0) throw new Error(`stickers.${repeat}: another sticker already uses this colour and symbol`);
    return { restartRequired: restart };
  }
  return undefined;
};

mount(SettingsOverlay, { target: document.querySelector("#fixture"), props: { onClose: () => {} } });
