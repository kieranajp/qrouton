import { AGENT_ATTENTION, AGENT_COVERAGE, AGENT_ROLES, AGENT_ROOT_ID, AGENT_STATES } from "../bridge/generated.js";

/** @type {Set<string>} */
const ROLES = new Set([AGENT_ROLES.ORCHESTRATOR, AGENT_ROLES.LEAD, AGENT_ROLES.SPECIALIST]);
/** @type {Set<string>} */
const STATES = new Set(Object.values(AGENT_STATES));

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

/** @param {string} role */
export function roleLabel(role) {
  return ROLES.has(role) ? role : "";
}

/** Only orchestrators can present a waiting-for-user state.
 * @param {string} state
 * @param {string} [role] */
export function stateLabel(state, role = "") {
  if (!STATES.has(state)) return "";
  if (state === AGENT_STATES.WAITING && role !== AGENT_ROLES.ORCHESTRATOR) return AGENT_STATES.WORKING;
  return state;
}

/** @param {AgentRecord} record */
export function activeAgent(record) {
  return WORKING.includes(record.state ?? "");
}

/** @param {AgentRecord} record */
export function finishedAgent(record) {
  return record.state === AGENT_STATES.FINISHED || record.state === AGENT_STATES.FAILED;
}

/** @param {AgentRecord} record */
export function runningRoot(record) {
  return (
    (record.role === AGENT_ROLES.ORCHESTRATOR || record.id === AGENT_ROOT_ID) &&
    activeAgent(record)
  );
}

/** @param {AgentRecord} record @param {{provider?: string, provider_label?: string}} [panel] */
export function recordLabel(record, panel = {}) {
  const role = roleLabel(record.role ?? "") || "Agent";
  const identity =
    record.role === AGENT_ROLES.ORCHESTRATOR
      ? providerLabel(record.provider || panel.provider, panel)
      : typeLabel(record.type ?? "");
  return [role, identity, stateLabel(record.state ?? "", record.role ?? "")]
    .filter(Boolean)
    .join(" · ");
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

/** @param {AgentRecord[]} records */
export function subagentTally(records = []) {
  const done = records.filter(finishedAgent).length;
  const plural = records.length === 1 ? "subagent" : "subagents";
  return `${records.length} ${plural} · ${done} done`;
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

  if (unseen > 0) facts.push({ kind: "unseen", label: `${unseen} unseen` });
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
 * type?: string, role?: string, state?: string, parent_known?: boolean,
 * started_at?: string, finished_at?: string}} AgentRecord */
/** @typedef {{record: AgentRecord, level: number, children: AgentNode[]}} AgentNode */

/** Parent-child relationships never cross provider runs.
 * @param {AgentRecord[]} records
 * @returns {{trees: AgentNode[], observed: AgentRecord[]}} */
export function projectAgents(records = []) {
  const nodes = records.map((record) => ({ record, level: 0, children: [] }));
  const key = (record) => `${record.provider ?? ""}\u0000${record.run_id ?? ""}\u0000${record.id ?? ""}`;
  const indexed = new Map(nodes.map((node) => [key(node.record), node]));
  const trees = nodes.filter(
    (node) => node.record.role === AGENT_ROLES.ORCHESTRATOR || node.record.id === AGENT_ROOT_ID,
  );
  for (const root of trees) root.level = 1;

  let changed = true;
  while (changed) {
    changed = false;
    for (const node of nodes) {
      if (node.level || !node.record.parent_known || !node.record.parent_id) continue;
      const parent = indexed.get(
        key({ ...node.record, id: node.record.parent_id }),
      );
      if (!parent?.level || parent.level >= 3 || parent === node) continue;
      node.level = parent.level + 1;
      parent.children.push(node);
      changed = true;
    }
  }

  const observed = nodes
    .filter((node) => !node.level)
    .map((node) => node.record);
  return { trees, observed };
}

/** Subagents remain collapsed beneath their lead until requested.
 * @param {AgentRecord[]} records */
export function hierarchy(records = []) {
  const { trees, observed } = projectAgents(records);
  return {
    roots: trees.map((root) => ({
      record: root.record,
      leads: root.children.map((lead) => ({
        record: lead.record,
        subagents: lead.children.map((node) => node.record),
      })),
    })),
    observed,
  };
}
