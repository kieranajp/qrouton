import { mount } from "svelte";
import { AGENT_TREE_EVENT, CHROME_AGENT_TREE } from "../src/lib/bridge/generated.js";
import { emitWailsEvent } from "./wails-runtime.js";
import AgentsFixture from "./AgentsFixture.svelte";

const node = (id, parent, extra = {}) => ({
  id,
  run: "run-1",
  parent,
  root: false,
  placed: true,
  type: "",
  description: "",
  state: "working",
  reason: "",
  tool: "",
  target: "",
  started: "2026-09-01T12:00:00Z",
  finished: "0001-01-01T00:00:00Z",
  toolCount: 0,
  tokens: 0,
  lastMessage: "",
  files: [],
  tools: [],
  ...extra,
});

const recorded = {
  provider: "claude",
  providerLabel: "Claude Code",
  coverage: "",
  nodes: [
    node("root", "", { root: true, state: "waiting_agents" }),
    node("a08c4b226d2052caf", "root", {
      type: "spike-b",
      description: "Spawn spike-a from spike-b",
      tool: "Agent",
      target: "spike-a",
      toolCount: 1,
      tools: [
        {
          tool: "Agent",
          target: "spike-a",
          start: "2026-09-01T12:00:05Z",
          end: "0001-01-01T00:00:00Z",
          failed: false,
        },
      ],
    }),
    node("ac6bcc1d7a22d6069", "a08c4b226d2052caf", {
      type: "spike-a",
      description: "Echo fg",
      state: "done",
      finished: "2026-09-01T12:00:20Z",
      tokens: 42191,
      toolCount: 1,
      lastMessage: "Done. Output was `fg`.",
      tools: [
        {
          tool: "Bash",
          target: "echo fg",
          start: "2026-09-01T12:00:10Z",
          end: "2026-09-01T12:00:11Z",
          failed: false,
        },
      ],
    }),
  ],
};

const opencode = {
  provider: "opencode",
  providerLabel: "OpenCode",
  coverage: "OpenCode reports the root agent only, so working and idle come from its terminal.",
  nodes: [node("root", "", { root: true, state: "idle" })],
};

const trees = { default: recorded, opencode };
const params = new URLSearchParams(location.search);
const tree = trees[params.get("tree") ?? "default"];

window.wailsCall = async (name) => (name === CHROME_AGENT_TREE ? tree : undefined);
window.pushTree = (key) => emitWailsEvent(AGENT_TREE_EVENT + "fixture", trees[key]);
window.trees = trees;

mount(AgentsFixture, { target: document.querySelector("#fixture") });
