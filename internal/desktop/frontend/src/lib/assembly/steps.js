// The chrome around the three steps, kept pure: node --test is the whole
// frontend harness. What each step is called, what stops it, and what a
// keypress in the dialog means.

/** @typedef {{field: string, message: string}} Problem */

const STEPS = [
  { label: "Describe the work", primary: "Choose repositories →" },
  { label: "Choose repositories", primary: "Choose an agent →" },
  { label: "Agent and mode", primary: "Create session →" },
];

export const labels = STEPS.map((step) => step.label);

export const last = STEPS.length - 1;

export const primary = (step = 0) => (STEPS[step] ?? STEPS[0]).primary;

// The fields a step is in a position to fix. A missing repository must not stop
// step 1, where there is nothing on screen to pick one with.
const OWNED = [
  ["name", "branchDescription", "ticket"],
  ["repos"],
  ["name", "branchDescription", "ticket", "repos"],
];

/**
 * @param {Problem[]} [problems]
 * @param {number} [step]
 * @returns {Problem | undefined}
 */
export const blocks = (problems = [], step = 0) =>
  problems.find((problem) => (OWNED[step] ?? []).includes(problem.field));

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

/** The empty-session fallback waits until the first payload settles.
 * @param {boolean} requested
 * @param {boolean} settled
 * @param {string} slug */
export const assemblyOpen = (requested, settled, slug) => !!settled && (!!requested || !slug);

/** An add-repositories picker closes when its originating session is no longer shown.
 * @param {string} shown
 * @param {boolean} pending
 * @param {string} added the session add-repos was pressed on */
export const pickerOpen = (shown, pending, added) => !!shown && (!!pending || added === shown);

/**
 * refusal is how the footer says what Go refused, which names the field before
 * the sentence.
 */
export const refusal = (err) => String(err?.message ?? err ?? "").replace(/^[a-z]+: /, "");

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
