import { AGENT_ATTENTION, AGENT_COVERAGE, AGENT_STATES } from "../bridge/generated.js";
import { stateWords } from "../overview/tree.js";

/** @typedef {{attention?: string, active?: number, coverage?: string, running?: boolean, tools?: boolean,
 * state?: string, reason?: string, tool?: string, target?: string}} Summary */
/** @typedef {{kind: string, label: string, active?: boolean, title?: string}} Fact */

/** @type {string[]} */
const BUSY = [AGENT_STATES.WORKING, AGENT_STATES.WAITING_AGENTS];

/** @param {string} words */
const capital = (words) => words.charAt(0).toUpperCase() + words.slice(1);

/** A row's headline: root state, live agents, current tool, unseen documents.
 * @param {Summary} summary @param {number} unseen @param {string} [idleAge] @returns {Fact[]} */
export function summaryFacts(summary = {}, unseen = 0, idleAge = "") {
  /** @type {Fact[]} */
  const facts = [];
  const state = summary.running ? (summary.state ?? "") : "";
  if (state === AGENT_STATES.NEEDS_YOU) {
    facts.push({ kind: "attention", label: "Needs you", title: summary.reason || undefined });
  } else if (summary.running && summary.attention === AGENT_ATTENTION.UNREAD) {
    facts.push({ kind: "attention", label: "Unread" });
  }

  if (!summary.running) {
    facts.push({ kind: "agents", label: idleAge ? `Idle · ${idleAge}` : "Idle" });
  } else if (state && state !== AGENT_STATES.NEEDS_YOU) {
    const agents = Math.max(0, (summary.active ?? 0) - 1);
    const count = summary.coverage === AGENT_COVERAGE.FULL && agents ? ` · ${agents} ${agents === 1 ? "agent" : "agents"}` : "";
    facts.push({
      kind: "agents",
      label: capital(stateWords({ state, tool: summary.tool }, summary.tools ?? true)) + count,
      active: BUSY.includes(state),
    });
  } else if (!state && summary.coverage === AGENT_COVERAGE.FULL) {
    facts.push({ kind: "agents", label: `${Math.max(0, summary.active ?? 0)} active`, active: true });
  } else if (!state && summary.coverage === AGENT_COVERAGE.ROOT) {
    facts.push({ kind: "agents", label: "Root active", active: true });
  }

  if (summary.running && summary.tool && state === AGENT_STATES.WORKING) {
    facts.push({ kind: "tool", label: [summary.tool, summary.target].filter(Boolean).join(" ") });
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
