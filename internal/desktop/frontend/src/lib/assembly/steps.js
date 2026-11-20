// The chrome around the three steps. What each step is called, what stops
// it, and what a keypress in the dialog means.

import { DRAFT_FIELDS } from "../bridge/generated.js";
import { fieldError } from "../fields.js";

/** @typedef {{field: import("../bridge/generated.js").DraftField, message: string}} Problem */

const DESCRIBED = [DRAFT_FIELDS.NAME, DRAFT_FIELDS.BRANCH_DESCRIPTION, DRAFT_FIELDS.TICKET];

/** @typedef {(typeof STEPS)[number]["key"]} Step */
const STEPS = /** @type {const} */ ([
  { key: "describe", label: "Describe the work", primary: "Choose repositories", owned: DESCRIBED },
  { key: "repositories", label: "Choose repositories", primary: "Choose an agent", owned: [DRAFT_FIELDS.REPOS] },
  { key: "agent", label: "Agent and mode", primary: "Create session", owned: [...DESCRIBED, DRAFT_FIELDS.REPOS] },
]);

export const labels = STEPS.map((step) => step.label);
export const last = STEPS.length - 1;
export const stepKey = (step = 0) => (STEPS[step] ?? STEPS[0]).key;
export const primary = (step = 0) => (STEPS[step] ?? STEPS[0]).primary;

/**
 * @param {Problem[]} [problems]
 * @param {number} [step]
 * @returns {Problem | undefined}
 */
export const blocks = (problems = [], step = 0) =>
  problems.find((problem) => (/** @type {readonly import("../bridge/generated.js").DraftField[]} */ (STEPS[step]?.owned ?? [])).includes(problem.field));

/** folder is the directory a branch names, which is the session's own slug. */
export const folder = (branch) => (branch ?? "").split("/").slice(1).join("/");

/**
 * @param {string} branch
 * @param {number} repos
 */
export function destination(branch, repos) {
  const into = folder(branch);
  if (!into) return "";
  return `${repos} repo${repos === 1 ? "" : "s"} into ${into}`;
}

/** @param {string} branch */
export const joining = (branch) => (branch ? `Added repositories join ${branch}` : "");

/** An install with no sessions has nothing to cancel back to, so the dialog is all it shows.
 * A shown session counts before the rail lists it.
 * @param {string} slug @param {number} sessions */
export const assemblyGated = (slug, sessions) => !slug && !sessions;

/** The empty-install fallback waits until the first payload settles.
 * @param {boolean} requested @param {boolean} settled @param {string} slug @param {number} sessions */
export const assemblyOpen = (requested, settled, slug, sessions) =>
  !!settled && (!!requested || assemblyGated(slug, sessions));

export const ADD_KEY = "add";

/** pickerKey is the request the overlay answers: an add keeps it until it ends, so
 * an escalation arriving mid-add waits its turn. Empty is no overlay.
 * @param {string} added @param {string} shown @param {string} pendingID */
export const pickerKey = (added, shown, pendingID) => {
  if (!shown) return "";
  if (added === shown) return ADD_KEY;
  return pendingID ?? "";
};

/** An add-repositories picker closes when its originating session is no longer shown.
 * @param {string} added
 * @param {string} shown
 * @param {string} pendingID */
export const pickerOpen = (added, shown, pendingID) => !!pickerKey(added, shown, pendingID);

/**
 * refusal is how the footer says what Go refused, which names the field before
 * the sentence.
 */
export const refusal = (err) => fieldError(err, DRAFT_FIELDS)?.message ?? String(err?.message ?? err ?? "");

/**
 * @param {{key?: string, target?: any}} [event]
 * @returns {'advance'|'cancel'|''}
 */
export function intent({ key, target } = {}) {
  if (key === "Escape") return "cancel";
  if (key !== "Enter") return "";
  const tag = (target?.tagName ?? "").toUpperCase();
  return tag === "TEXTAREA" || tag === "BUTTON" ? "" : "advance";
}
