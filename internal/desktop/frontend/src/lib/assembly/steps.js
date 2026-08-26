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
  ["name", "ticket"],
  ["repos"],
  ["name", "ticket", "repos"],
];

/**
 * blocks is the problem stopping a step, which is both what the footer says and
 * why the primary button did nothing.
 * @param {Problem[]} [problems]
 * @param {number} [step]
 * @returns {Problem | undefined}
 */
export const blocks = (problems = [], step = 0) =>
  problems.find((problem) => (OWNED[step] ?? []).includes(problem.field));

/** folder is the directory a branch names, which is the session's own slug. */
export const folder = (branch) => (branch ?? "").split("/").slice(1).join("/");

/**
 * destination is the last thing the footer says before Create: how much is
 * going where.
 * @param {string} branch
 * @param {number} repos
 */
export function destination(branch, repos) {
  const into = folder(branch);
  if (!into) return "";
  return `${repos} repo${repos === 1 ? "" : "s"} into ${into}`;
}

/**
 * joining is what the picker's footer says: the branch anything added lands on.
 * A session with no repositories yet has no branch, and nothing to say.
 * @param {string} branch
 */
export const joining = (branch) => (branch ? `Added repositories join ${branch}` : "");

/**
 * assemblyOpen is whether the assembly overlay is drawn: asked for by the rail's
 * button, or because the window holds no session and so has nothing else to
 * offer. It waits for the first payload, since an unsettled window reads as
 * having no session too.
 * @param {boolean} requested
 * @param {boolean} settled
 * @param {string} slug
 */
export const assemblyOpen = (requested, settled, slug) => !!settled && (!!requested || !slug);

/**
 * pickerOpen is whether the picker is drawn over the session on screen: an
 * escalation waiting on it, or the add-repos button pressed on that same
 * session. Pressing it on one session and switching to another closes it.
 * @param {string} shown
 * @param {boolean} pending
 * @param {string} added the session add-repos was pressed on
 */
export const pickerOpen = (shown, pending, added) => !!shown && (!!pending || added === shown);

/**
 * refusal is how the footer says what Go refused, which names the field before
 * the sentence.
 */
export const refusal = (err) => String(err?.message ?? err ?? "").replace(/^[a-z]+: /, "");

/**
 * intent is what a keypress in the dialog means. Enter advances, except while
 * typing prose or pressing a control that answers for itself.
 * @param {{key?: string, target?: any}} [event]
 * @returns {'advance'|'cancel'|''}
 */
export function intent({ key, target } = {}) {
  if (key === "Escape") return "cancel";
  if (key !== "Enter") return "";
  const tag = (target?.tagName ?? "").toUpperCase();
  return tag === "TEXTAREA" || tag === "BUTTON" ? "" : "advance";
}
