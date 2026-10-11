import { AGENT_ATTENTION, AGENT_COVERAGE } from "../bridge/generated.js";

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
