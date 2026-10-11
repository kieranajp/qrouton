import { AGENT_STAGES, AGENT_STATES } from "../bridge/generated.js";

/** @typedef {{tool: string, target: string, start: string, end: string, failed: boolean, abandoned?: boolean}} AgentTool */
/** @typedef {{name: string, path: string, kind: string, id: string}} AgentDocument */
/** @typedef {{id: string, run: string, parent: string, root: boolean, placed: boolean, type: string, stage?: string,
 * description: string, state: string, reason: string, tool: string, target: string, started: string, finished: string,
 * toolCount: number, tokens: number, lastMessage: string, files: string[], documents?: AgentDocument[],
 * tools: AgentTool[]}} AgentNode */
/** @typedef {{provider: string, providerLabel: string, coverage: string, tools: boolean, nodes: AgentNode[]}} AgentTree */
/** @typedef {{node: AgentNode, children: Branch[]}} Branch */
/** @typedef {{run: string, root: AgentNode | undefined, children: Branch[]}} Run */

const SECOND = 1000;
const MINUTE = 60 * SECOND;
const HOUR = 60 * MINUTE;

/** @type {Set<string>} */
const FINISHED = new Set([AGENT_STATES.DONE, AGENT_STATES.FAILED, AGENT_STATES.ENDED]);

/** @param {string | undefined} state */
export const isFinished = (state) => FINISHED.has(state ?? "");

/** @param {string | undefined} stamp */
function millis(stamp) {
  const time = Date.parse(stamp ?? "");
  return Number.isFinite(time) && time > Date.parse("1970-01-01T00:00:00Z") ? time : 0;
}

/** @param {Branch} a @param {Branch} b */
function byLiveThenStart(a, b) {
  const live = Number(isFinished(a.node.state)) - Number(isFinished(b.node.state));
  if (live) return live;
  return (millis(a.node.started) || Infinity) - (millis(b.node.started) || Infinity);
}

/** @param {Branch} branch */
function sortBranch(branch) {
  branch.children.sort(byLiveThenStart);
  branch.children.forEach(sortBranch);
}

/** @param {AgentNode[]} nodes @param {string} run @returns {Run} */
function buildRun(nodes, run) {
  const root = nodes.find((node) => node.root);
  const branches = new Map(nodes.map((node) => [node.id, { node, children: [] }]));
  const rootBranch = root && branches.get(root.id);
  /** @type {Map<string, string>} */
  const parentOf = new Map();
  for (const node of nodes) {
    if (node === root) continue;
    let parent = branches.has(node.parent) && node.parent !== node.id ? node.parent : (root?.id ?? "");
    for (let up = parent, steps = 0; up && up !== root?.id; up = parentOf.get(up) ?? "", steps++) {
      if (up === node.id || steps > nodes.length) {
        parent = root?.id ?? "";
        break;
      }
    }
    parentOf.set(node.id, parent);
  }
  /** @type {Branch[]} */
  const top = [];
  for (const node of nodes) {
    if (node === root) continue;
    const branch = branches.get(node.id);
    const parent = branches.get(parentOf.get(node.id) ?? "");
    if (branch) (parent ?? { children: top }).children.push(branch);
  }
  const children = rootBranch ? rootBranch.children : top;
  const holder = { node: root ?? nodes[0], children };
  sortBranch(holder);
  return { run, root, children: holder.children };
}

/** Runs ordered current first: the run whose root has not finished, else the latest root start.
 * @param {AgentNode[]} nodes @returns {Run[]} */
export function buildRuns(nodes = []) {
  /** @type {Map<string, AgentNode[]>} */
  const groups = new Map();
  for (const node of nodes) groups.set(node.run, [...(groups.get(node.run) ?? []), node]);
  const runs = [...groups].map(([run, members]) => buildRun(members, run));
  const startOf = (/** @type {Run} */ run) => millis(run.root?.started);
  const liveOf = (/** @type {Run} */ run) => Number(!isFinished(run.root?.state));
  return runs.sort((a, b) => liveOf(b) - liveOf(a) || startOf(b) - startOf(a));
}

const WORDS = {
  [AGENT_STATES.WAITING_AGENTS]: "waiting on agents",
  [AGENT_STATES.NEEDS_YOU]: "needs you",
  [AGENT_STATES.YOUR_TURN]: "your turn",
  [AGENT_STATES.IDLE]: "idle",
  [AGENT_STATES.DONE]: "done",
  [AGENT_STATES.FAILED]: "failed",
  [AGENT_STATES.ENDED]: "ended with run",
};

/** Between tool calls is thinking, which only a runner reporting its tool calls can tell.
 * @param {{state: string, tool?: string}} node @param {boolean} [tools] */
export function stateWords(node, tools = true) {
  if (node.state === AGENT_STATES.WORKING) return node.tool || !tools ? "working" : "thinking";
  return WORDS[node.state] ?? node.state;
}

/** @param {string} state @returns {string} the state icon, or "" for none */
export function stateIcon(state) {
  switch (state) {
    case AGENT_STATES.WORKING:
    case AGENT_STATES.WAITING_AGENTS:
      return "running";
    case AGENT_STATES.NEEDS_YOU:
      return "waiting";
    case AGENT_STATES.FAILED:
      return "failed";
    case AGENT_STATES.DONE:
    case AGENT_STATES.ENDED:
      return "succeeded";
    default:
      return "";
  }
}

/** @param {string} state */
export function stateTone(state) {
  switch (state) {
    case AGENT_STATES.WORKING:
    case AGENT_STATES.WAITING_AGENTS:
      return "--state-running";
    case AGENT_STATES.NEEDS_YOU:
      return "--state-waiting";
    case AGENT_STATES.YOUR_TURN:
      return "--accent-action";
    case AGENT_STATES.FAILED:
      return "--state-failed";
    default:
      return "--text-faint";
  }
}

/** @param {number} span milliseconds */
function spanLabel(span) {
  if (span < MINUTE) return `${Math.floor(span / SECOND)}s`;
  if (span < HOUR) {
    const minutes = Math.floor(span / MINUTE);
    const seconds = Math.floor((span % MINUTE) / SECOND);
    return seconds ? `${minutes}m ${seconds}s` : `${minutes}m`;
  }
  const hours = Math.floor(span / HOUR);
  const minutes = Math.floor((span % HOUR) / MINUTE);
  return minutes ? `${hours}h ${minutes}m` : `${hours}h`;
}

/** @param {string} started @param {string} finished @param {number} now */
export function elapsed(started, finished, now) {
  const from = millis(started);
  if (!from) return "";
  const span = (millis(finished) || now) - from;
  return span < 0 ? "" : spanLabel(span);
}

/** @param {number} tokens */
export function tokenLabel(tokens) {
  if (!tokens || tokens < 0) return "";
  if (tokens < 1000) return `${tokens} tokens`;
  return `${(tokens / 1000).toFixed(1)}k tokens`;
}

/** An agent's name: its type, or what it was spawned to do when no type
 * reached the workbench.
 * @param {{type?: string, description?: string}} node */
export const agentName = (node) => typeLabel(node.type ?? "") || node.description || "Agent";

/** @param {string} type */
export function typeLabel(type) {
  return type
    .trim()
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((word) => word[0].toUpperCase() + word.slice(1))
    .join(" ");
}

/** @param {{tool?: string, target?: string}} node */
export const toolLine = (node) => [node.tool, node.target].filter(Boolean).join(" ");

/** @type {string[]} */
const STAGE_ORDER = [AGENT_STAGES.RESEARCH, AGENT_STAGES.PLAN, AGENT_STAGES.IMPLEMENT];

export const STAGE_NAMES = {
  [AGENT_STAGES.RESEARCH]: "Research",
  [AGENT_STAGES.PLAN]: "Plan",
  [AGENT_STAGES.IMPLEMENT]: "Implementation",
};

const STAGE_TONES = {
  [AGENT_STAGES.RESEARCH]: "RESEARCH",
  [AGENT_STAGES.PLAN]: "PLAN",
  [AGENT_STAGES.IMPLEMENT]: "IMPLEMENT",
};

/** @param {string | undefined} stage */
export const stageName = (stage) => STAGE_NAMES[stage ?? ""] ?? "";

/** The workflow stage a lead's square wears the tone of, for workflowTone.
 * @param {string | undefined} stage @returns {"RESEARCH" | "PLAN" | "IMPLEMENT" | undefined} */
export const stageTone = (stage) => STAGE_TONES[stage ?? ""];

/** @param {Branch} branch */
const rank = (branch) => {
  const at = STAGE_ORDER.indexOf(branch.node.stage ?? "");
  return at < 0 ? STAGE_ORDER.length : at;
};

/** The root's direct children are its leads, in research, plan, implement order, then by
 * start; an earlier run's leads are earlier rows of the same list.
 * @param {AgentNode[]} nodes @returns {{root: AgentNode | undefined, leads: Branch[]}} */
export function overview(nodes = []) {
  const runs = buildRuns(nodes);
  const leads = runs.flatMap((run) => run.children);
  leads.sort((a, b) => rank(a) - rank(b) || millis(a.node.started) - millis(b.node.started));
  return { root: runs[0]?.root, leads };
}

/** @param {AgentNode} root @param {Branch[]} leads @param {boolean} [tools] */
export function rootWords(root, leads, tools = true) {
  if (root.state === AGENT_STATES.WAITING_AGENTS) {
    const live = leads.find((lead) => !isFinished(lead.node.state) && stageName(lead.node.stage));
    if (live) return `waiting on ${stageName(live.node.stage)}`;
  }
  return stateWords(root, tools);
}

/** A path inside the session reads from its repository down.
 * @param {string} path @param {string} [root] */
export function sessionPath(path, root = "") {
  if (!root || !path.startsWith(root + "/")) return path;
  const rest = path.slice(root.length + 1);
  return rest.startsWith("src/") ? rest.slice(4) : rest;
}

/** @param {{tool?: string, target?: string}} node @param {string} [root] */
export function nowLine(node, root = "") {
  return [node.tool, sessionPath(node.target ?? "", root)].filter(Boolean).join(" ");
}

/** @param {string[]} files */
export const wroteLine = (files = []) => files.map((file) => file.split("/").pop()).join(", ");
