import { AGENT_ATTENTION, AGENT_COVERAGE, AGENT_ROOT_ID, AGENT_STATES } from "../bridge/generated.js";

/** @type {Set<string>} */
const STATES = new Set(Object.values(AGENT_STATES));

/** @type {string[]} */
const FINISHED = [AGENT_STATES.FINISHED, AGENT_STATES.FAILED, AGENT_STATES.ENDED];

/** @type {string[]} */
const WORKING = [AGENT_STATES.WAITING, AGENT_STATES.WORKING, AGENT_STATES.IDLE, AGENT_STATES.ACTIVE];

/** @param {string} value */
function humanize(value) {
  return value
    .trim()
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((word) => word[0].toUpperCase() + word.slice(1))
    .join(" ");
}

/** Go names the session's own runner; any other provider is only humanized.
 * @param {string} provider
 * @param {{provider?: string, provider_label?: string}} [panel] */
export function providerLabel(provider, panel = {}) {
  if (provider && provider === panel.provider && panel.provider_label) return panel.provider_label;
  return provider ? humanize(provider) : "";
}

// A name qrouton cannot read is a line it does not draw. Every label below
// answers with nothing rather than with a placeholder sentence.

/** @param {string} type */
export function typeLabel(type) {
  return type ? humanize(type) : "";
}

/** @param {AgentRecord} record */
export const isRoot = (record) => record.id === AGENT_ROOT_ID;

/** Only the root can present a waiting-for-user state.
 * @param {string} state
 * @param {boolean} [root] */
export function stateLabel(state, root = false) {
  if (!STATES.has(state)) return "";
  if (state === AGENT_STATES.WAITING && !root) return AGENT_STATES.WORKING;
  return state;
}

/** @param {AgentRecord} record */
export function activeAgent(record) {
  return WORKING.includes(record.state ?? "");
}

/** @param {AgentRecord} record */
export function finishedAgent(record) {
  return FINISHED.includes(record.state ?? "");
}

/** @param {AgentRecord} record */
export function runningRoot(record) {
  return isRoot(record) && activeAgent(record);
}

/** @param {AgentRecord} record @param {{provider?: string, provider_label?: string}} [panel] */
export function recordLabel(record, panel = {}) {
  const identity = isRoot(record)
    ? ["Orchestrator", providerLabel(record.provider || panel.provider, panel)]
    : [typeLabel(record.type ?? "") || "Agent"];
  return [...identity, stateLabel(record.state ?? "", isRoot(record))].filter(Boolean).join(" · ");
}

const SECOND = 1000;
const MINUTE = 60 * SECOND;
const HOUR = 60 * MINUTE;

/**
 * @param {AgentRecord} record
 * @param {number} [now]
 */
export function duration(record, now = Date.now()) {
  const started = new Date(record.started_at ?? 0).getTime();
  if (!Number.isFinite(started) || started <= 0) return "";
  const until = finishedAgent(record) ? new Date(record.finished_at ?? 0).getTime() : now;
  const span = (Number.isFinite(until) && until > 0 ? until : now) - started;
  if (span < 0) return "";
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

/**
 * @param {{attention?: string, active?: number, coverage?: string, running?: boolean}} summary
 * @param {number} unseen
 * @param {string} [idleAge]
 */
export function summaryFacts(summary = {}, unseen = 0, idleAge = "") {
  const facts = [];
  if (summary.running && summary.attention === AGENT_ATTENTION.UNREAD) {
    facts.push({ kind: "attention", label: "Unread" });
  }

  if (!summary.running) {
    facts.push({ kind: "agents", label: idleAge ? `Idle · ${idleAge}` : "Idle" });
  } else if (summary.coverage === AGENT_COVERAGE.FULL) {
    facts.push({ kind: "agents", label: `${Math.max(0, summary.active ?? 0)} active`, active: true });
  } else if (summary.coverage === AGENT_COVERAGE.ROOT) {
    facts.push({ kind: "agents", label: "Root active", active: true });
  }

  if (unseen > 0) facts.push({ kind: "unseen", label: `${unseen} new ${unseen === 1 ? "doc" : "docs"}` });
  return facts;
}

/** @param {{name?: string}[]} repos */
export function repositoryLine(repos = []) {
  if (!repos.length) return { name: "No editing repositories", extra: "" };
  return { name: repos[0].name ?? "", extra: repos.length > 1 ? `+${repos.length - 1}` : "" };
}

/**
 * @param {string} name
 * @param {{name?: string}[]} repos
 * @param {{label: string}[]} facts
 */
export function rowLabel(name, repos, facts) {
  const line = repositoryLine(repos);
  return [name, [line.name, line.extra].filter(Boolean).join(" "), ...facts.map((fact) => fact.label)]
    .filter(Boolean)
    .join(" · ");
}

/** @param {{provider?: string, provider_label?: string, children_known?: boolean}} panel */
export function capabilityNote(panel = {}) {
  if (!panel.provider) return "Provider unknown · live activity unavailable";
  if (panel.children_known) return "";
  return `${providerLabel(panel.provider, panel)} provides root activity only.`;
}

/** @typedef {{id?: string, run_id?: string, provider?: string, parent_id?: string,
 * type?: string, description?: string, state?: string, parent_known?: boolean,
 * started_at?: string, finished_at?: string}} AgentRecord */
/** @typedef {{record: AgentRecord, level: number, children: AgentNode[]}} AgentNode */

/** Places every agent under its parent at any depth. Parents never cross runs;
 * an agent with no known parent, or one whose parent is gone, is observed.
 * @param {AgentRecord[]} records
 * @returns {{trees: AgentNode[], observed: AgentRecord[]}} */
export function projectAgents(records = []) {
  const key = (run, id) => `${run ?? ""}\u0000${id ?? ""}`;
  /** @type {AgentNode[]} */
  const nodes = records.map((record) => ({ record, level: 0, children: [] }));
  const indexed = new Map(nodes.map((node) => [key(node.record.run_id, node.record.id), node]));
  const trees = nodes.filter((node) => isRoot(node.record));
  const placed = new Set(trees);
  for (const node of nodes) {
    if (placed.has(node) || !node.record.parent_known || !node.record.parent_id) continue;
    const parent = indexed.get(key(node.record.run_id, node.record.parent_id));
    if (parent && parent !== node) parent.children.push(node);
  }
  /** @param {AgentNode} node @param {number} level */
  const descend = (node, level) => {
    node.level = level;
    placed.add(node);
    for (const child of node.children) if (!placed.has(child)) descend(child, level + 1);
  };
  for (const root of trees) descend(root, 1);
  for (const node of nodes) node.children = node.children.filter((child) => child.level === node.level + 1);
  return { trees, observed: nodes.filter((node) => !placed.has(node)).map((node) => node.record) };
}

/** Flattens a tree into rows in display order, each carrying its depth.
 * @param {AgentNode[]} trees @returns {AgentNode[]} */
export function rows(trees = []) {
  /** @type {AgentNode[]} */
  const out = [];
  /** @param {AgentNode} node */
  const walk = (node) => {
    out.push(node);
    node.children.forEach(walk);
  };
  trees.forEach(walk);
  return out;
}
