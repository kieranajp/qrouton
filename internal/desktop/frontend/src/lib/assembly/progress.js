// Assembly progress as rows on screen, kept pure: node --test is the whole
// frontend harness.

/** @typedef {'pending'|'running'|'done'|'failed'} State */
const STATES = { started: "running", advanced: "running", completed: "done", failed: "failed" };

/**
 * @typedef {object} Event
 * @property {string} step
 * @property {string} status
 * @property {string} [repo] the `org/name` Go names it by
 * @property {string} [phase]
 * @property {number} [percent]
 * @property {string} [error]
 */
/**
 * @typedef {object} Row
 * @property {string} step
 * @property {string} repo
 * @property {string} status
 * @property {State} state
 * @property {string} label
 * @property {string} detail
 * @property {number} [percent]
 */

/** Progress replaces the current step row, while each outcome starts a new row.
 * @param {Row[]} rows
 * @param {Event} event
 * @returns {Row[]} */
export function record(rows, event) {
  const row = toRow(event);
  const last = rows[rows.length - 1];
  if (row.status === "advanced" && last?.status === "advanced" && sameStep(last, row)) {
    return [...rows.slice(0, -1), row];
  }
  return [...rows, row];
}

const sameStep = (a, b) => a.step === b.step && a.repo === b.repo;

/** @returns {Row} */
function toRow(event) {
  const repo = event.repo ?? "";
  return {
    step: event.step,
    repo,
    status: event.status,
    state: STATES[event.status] ?? "pending",
    label: repo ? `${repo} ${event.step}` : event.step,
    detail: (event.status === "failed" ? event.error : event.phase) ?? "",
    percent: event.status === "advanced" ? event.percent : undefined,
  };
}
