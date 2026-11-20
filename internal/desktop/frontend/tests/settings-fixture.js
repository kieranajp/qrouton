import "../src/tokens/scale.css";
import { mount } from "svelte";
import { emitWailsEvent } from "./wails-runtime.js";
import { STICKER_DUPLICATE, UI_SCALE_EVENT, WINDOW_EXIT_EVENT, WINDOWS_EVENT } from "../src/lib/bridge/generated.js";
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
const calls = [];
let closes = 0;

// ?shown=1 puts a session on screen for Sign in; ?credential=1 sets ANTHROPIC_API_KEY;
// ?uncounted=1 is a sessions root that could not be read; ?chooser=off is a client with no folder picker.
const shown = params.get("shown") === "1";
const chooser = params.get("chooser") !== "off";
// Each folder pick takes the next queued answer: a path, "" for a cancel, or an Error the picker throws.
const folders = [];
// A held method answers only once the test releases it.
const held = new Map();
const uncounted = params.get("uncounted") === "1";
const credential = params.get("credential") === "1";
const profiles = {
  claude: [{ id: "work", label: "Lifesum", dir: "/home/k/.claude-work", sessions: uncounted ? 0 : 2, uncounted }],
  codex: [],
};
const profileCalls = [];
const slug = (label) => label.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
const profilesView = () => ({
  sessionShown: shown,
  canChooseFolder: chooser,
  runners: [
    {
      id: "claude",
      label: "Claude Code",
      credentials: credential ? ["ANTHROPIC_API_KEY"] : [],
      warning: credential
        ? "Every Claude Code profile signs in with ANTHROPIC_API_KEY from qrouton's environment, not its own stored login."
        : undefined,
      profiles: profiles.claude.map((row) => ({ ...row })),
    },
    { id: "codex", label: "Codex CLI", credentials: [], profiles: profiles.codex.map((row) => ({ ...row })) },
  ],
});
const profilesCall = async (method, args) => {
  profileCalls.push({ method, args });
  await held.get(method)?.promise;
  if (method === "View") return profilesView();
  if (method === "Suggest") {
    const [runner, label] = args;
    const id = slug(label);
    return id ? { id, dir: `~/.${runner}-${id}` } : { id: "", dir: "" };
  }
  if (method === "Add") {
    const [input] = args;
    const id = slug(input.label);
    profiles[input.runner].push({ id, label: input.label.trim(), dir: input.dir, sessions: 0 });
    return profilesView();
  }
  if (method === "Rename") {
    const [runner, id, label] = args;
    profiles[runner].find((row) => row.id === id).label = label;
    return profilesView();
  }
  if (method === "Delete") {
    const [runner, id] = args;
    profiles[runner] = profiles[runner].filter((row) => row.id !== id);
    return profilesView();
  }
  if (method === "ChooseFolder") {
    const next = folders.shift() ?? "";
    if (next instanceof Error) throw next;
    return next;
  }
  if (method === "Status")
    return [
      { runner: "claude", profile: "", state: "signed-in", email: "k@example.com" },
      { runner: "claude", profile: "work", state: "signed-out" },
      { runner: "codex", profile: "", state: "unknown" },
    ];
  if (method === "SignIn") return new Promise((resolve) => setTimeout(() => resolve("window-9"), 100));
  return undefined;
};

window.settingsFixture = {
  queueFolders: (...answers) => folders.push(...answers.map((answer) => (answer?.error ? new Error(answer.error) : answer))),
  hold: (method) => {
    let release = () => {};
    const promise = new Promise((resolve) => (release = resolve));
    held.set(method, { promise, release });
  },
  release: (method) => {
    held.get(method)?.release();
    held.delete(method);
  },
  profileCalls: () => profileCalls.map((call) => ({ ...call })),
  endSignIn: (id) => {
    emitWailsEvent(WINDOW_EXIT_EVENT + id, 0);
    emitWailsEvent(WINDOWS_EVENT, { session: "octopus", selected: id, tabs: [{ id, status: "succeeded" }] });
  },
  saves: () => [...saves],
  calls: () => [...calls],
  closes: () => closes,
  announceScale: (percent) => emitWailsEvent(UI_SCALE_EVENT, percent),
};

window.wailsCall = async (name, ...args) => {
  const [input] = args;
  calls.push(name.split(".").pop());
  if (name.includes(".Profiles.")) return profilesCall(name.split(".").pop(), args);
  if (name.endsWith("Placement.Check")) window.placementChecks = (window.placementChecks ?? 0) + 1;
  if (failing && name.endsWith("." + failing)) throw new Error("config.json: permission denied");
  if (name.endsWith("Ollama.Check")) return ollama;
  if (name.endsWith(".GitHubStatus")) {
    const check = gh === "cli" || gh === "env" ? ghCheck : "";
    return { state: gh, check, login: check === "valid" ? "kieranajp" : "", variable: "GITHUB_TOKEN" };
  }
  if (name.endsWith(".Load"))
    return {
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
mount(SettingsOverlay, { target: document.querySelector("#fixture"), props: { onClose: () => closes++ } });
