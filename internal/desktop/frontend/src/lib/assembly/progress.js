// Assembly progress as rows on screen.

import { PROGRESS_STATUSES } from "../bridge/generated.js";

/** @typedef {'pending'|'running'|'done'|'failed'} State */
const STATES = {
  [PROGRESS_STATUSES.STARTED]: "running",
  [PROGRESS_STATUSES.ADVANCED]: "running",
  [PROGRESS_STATUSES.COMPLETED]: "done",
  [PROGRESS_STATUSES.FAILED]: "failed",
};

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
  if (row.status === PROGRESS_STATUSES.ADVANCED && last?.status === PROGRESS_STATUSES.ADVANCED && sameStep(last, row)) {
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
    detail: (event.status === PROGRESS_STATUSES.FAILED ? event.error : event.phase) ?? "",
    percent: event.status === PROGRESS_STATUSES.ADVANCED ? event.percent : undefined,
  };
}
