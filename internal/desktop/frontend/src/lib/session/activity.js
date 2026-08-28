const ROLES = new Set(["Orchestrator", "Lead", "Specialist"]);
const STATES = new Set([
  "Waiting for you",
  "Working",
  "Idle",
  "Active",
  "Finished",
  "Failed",
]);

/** @param {string[]} values */
function sentenceList(values) {
  if (values.length < 2) return values[0] ?? "";
  if (values.length === 2) return values.join(" and ");
  return `${values.slice(0, -1).join(", ")}, and ${values.at(-1)}`;
}

/** @param {string} value */
function humanize(value) {
  return value
    .trim()
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((word) => (word.toLowerCase() === "qrspi" ? "QRSPI" : word[0].toUpperCase() + word.slice(1)))
    .join(" ");
}

/** @param {string} provider */
export function providerLabel(provider) {
  switch (provider?.toLowerCase()) {
    case "claude":
      return "Claude";
    case "codex":
      return "Codex";
    case "opencode":
      return "OpenCode";
    default:
      return provider ? humanize(provider) : "Provider unknown";
  }
}

/** @param {string} type */
export function typeLabel(type) {
  return type ? humanize(type) : "Type unavailable";
}

/** @param {string} role */
export function roleLabel(role) {
  return ROLES.has(role) ? role : "Role unavailable";
}

/** @param {string} state */
export function stateLabel(state) {
  return STATES.has(state) ? state : "State unavailable";
}

/** @param {AgentRecord} record */
export function parentLabel(record) {
  return record.parent_known && record.parent_id
    ? `Parent ${record.parent_id}`
    : "Parent unavailable";
}

/** @param {AgentRecord} record */
export function runningRoot(record) {
  return (
    (record.role === "Orchestrator" || record.id === "root") &&
    ["Waiting for you", "Working", "Idle", "Active"].includes(record.state ?? "")
  );
}

/** @param {AgentRecord} record @param {string} fallbackProvider */
export function recordLabel(record, fallbackProvider = "") {
  const role = roleLabel(record.role ?? "");
  const identity =
    role === "Orchestrator"
      ? providerLabel(record.provider || fallbackProvider)
      : typeLabel(record.type ?? "");
  return `${role} · ${identity} · ${stateLabel(record.state ?? "")}`;
}

/**
 * @param {{attention?: string, active?: number, coverage?: string, running?: boolean}} summary
 * @param {number} unseen
 */
export function summaryFacts(summary = {}, unseen = 0) {
  const facts = [];
  if (summary.running && summary.attention === "needs-you") {
    facts.push({ kind: "attention", label: "Needs you" });
  }

  if (!summary.running) {
    facts.push({ kind: "agents", label: "Not running" });
  } else if (summary.coverage === "full") {
    facts.push({ kind: "agents", label: `${Math.max(0, summary.active ?? 0)} active`, active: true });
  } else if (summary.coverage === "root") {
    facts.push({ kind: "agents", label: "Root active", active: true });
  } else {
    facts.push({ kind: "agents", label: "Activity unavailable" });
  }

  if (unseen > 0) facts.push({ kind: "unseen", label: `${unseen} unseen` });
  return facts;
}

/** @param {{name?: string}[]} repos */
export function repositoryLine(repos = []) {
  return repos.length ? repos.map((repo) => repo.name).join(" · ") : "No editing repositories";
}

/**
 * @param {string} name
 * @param {{name?: string}[]} repos
 * @param {{label: string}[]} facts
 */
export function rowLabel(name, repos, facts) {
  return [name, repositoryLine(repos), ...facts.map((fact) => fact.label)].filter(Boolean).join(" · ");
}

/**
 * @param {{provider?: string, attention_known?: boolean, children_known?: boolean,
 * parents_known?: boolean, outcomes_known?: boolean}} panel
 */
export function capabilityNote(panel = {}) {
  if (!panel.provider) return "Provider unknown · live activity unavailable";
  const unavailable = [];
  if (!panel.attention_known) unavailable.push("attention");
  if (!panel.children_known) unavailable.push("delegated agents");
  if (!panel.parents_known) unavailable.push("parent relationships");
  if (!panel.outcomes_known) unavailable.push("outcomes");
  if (!unavailable.length) return "";
  const prefix = panel.children_known
    ? ""
    : `${providerLabel(panel.provider)} provides root activity only. `;
  const missing = sentenceList(unavailable);
  const sentence = missing[0].toUpperCase() + missing.slice(1);
  return `${prefix}${sentence} unavailable.`;
}

/** @typedef {{id?: string, run_id?: string, provider?: string, parent_id?: string,
 * type?: string, role?: string, state?: string, parent_known?: boolean}} AgentRecord */
/** @typedef {{record: AgentRecord, level: number, children: AgentNode[]}} AgentNode */

/**
 * Projects only relationships whose exact parent record is present in the same provider run.
 * @param {AgentRecord[]} records
 * @returns {{trees: AgentNode[], observed: AgentRecord[]}}
 */
export function projectAgents(records = []) {
  const nodes = records.map((record) => ({ record, level: 0, children: [] }));
  const key = (record) => `${record.provider ?? ""}\u0000${record.run_id ?? ""}\u0000${record.id ?? ""}`;
  const indexed = new Map(nodes.map((node) => [key(node.record), node]));
  const trees = nodes.filter(
    (node) => node.record.role === "Orchestrator" || node.record.id === "root",
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
