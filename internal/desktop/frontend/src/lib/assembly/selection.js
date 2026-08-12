// Which repositories a session takes and in what order, kept pure: node --test
// is the whole frontend harness.

import { repoID } from "./filter.js";

/** @typedef {'off'|'editing'|'reference'} Role */
/**
 * @typedef {object} Selection
 * @property {string[]} order the ids in the order they were picked, which is the ranking
 * @property {Record<string, Role>} roles
 * @property {string[]} locked the ids the session already holds
 */

const GLYPHS = { editing: "●", reference: "◐" };
const READ_ONLY = "read-only";
const IN_SESSION = "in session";

/**
 * seed is the selection step 2 opens with. A held row carries its role and stays
 * out of the order: it reaches Go as a manifest entry to leave alone, and
 * composing it again clones a repository the agent is already working in.
 * @param {{id: string, role: Role}[]} [held]
 * @returns {Selection}
 */
export function seed(held = []) {
  /** @type {Record<string, Role>} */
  const roles = {};
  for (const row of held) roles[row.id] = row.role;
  return { order: [], roles, locked: held.map((row) => row.id) };
}

/** @returns {Role} */
export const roleOf = (selection, id) => selection.roles[id] ?? "off";

export const isLocked = (selection, id) => selection.locked.includes(id);

/**
 * rowMeta says in words what the role column cannot: a held row wears the role
 * it has, so the toggle beside it looks exactly like one that would answer.
 * @param {string} meta
 * @param {boolean} locked
 */
export const rowMeta = (meta, locked) =>
  locked ? [meta, IN_SESSION].filter(Boolean).join(" · ") : meta;

/**
 * setRole records one row's role. A repository keeps the rank it was first given,
 * so demoting it to reference holds its place while turning it off and back sends
 * it last.
 * @returns {Selection}
 */
export function setRole(selection, id, role) {
  if (isLocked(selection, id)) return selection;
  const roles = { ...selection.roles };
  if (role === "off") {
    delete roles[id];
    return { ...selection, roles, order: selection.order.filter((seen) => seen !== id) };
  }
  roles[id] = role;
  const order = selection.order.includes(id) ? selection.order : [...selection.order, id];
  return { ...selection, roles, order };
}

/**
 * reconcile drops the repositories a refreshed list no longer carries. Held rows
 * survive it, because the session holds them whatever GitHub now reports.
 * @param {Selection} selection
 * @param {string[]} ids
 * @returns {Selection}
 */
export function reconcile(selection, ids) {
  const available = new Set(ids);
  /** @type {Record<string, Role>} */
  const roles = {};
  for (const [id, role] of Object.entries(selection.roles)) {
    if (available.has(id) || isLocked(selection, id)) roles[id] = role;
  }
  return { ...selection, roles, order: selection.order.filter((id) => available.has(id)) };
}

/** counts is the `2 editing · 1 reference` line, which describes the rows on screen. */
export function counts(selection) {
  let editing = 0;
  let reference = 0;
  for (const role of Object.values(selection.roles)) {
    if (role === "editing") editing++;
    else if (role === "reference") reference++;
  }
  return { editing, reference };
}

/**
 * ordered is what Go composes, in rank order.
 * @returns {{id: string, role: Role}[]}
 */
export const ordered = (selection) =>
  selection.order.map((id) => ({ id, role: selection.roles[id] }));

/**
 * summary is the selected chips. An editing chip names the branch it joins, and
 * says nothing while there is no branch to name.
 * @param {Selection} selection
 * @param {{org: string, name: string, default_branch?: string}[]} repos
 * @param {string} branch
 * @returns {{id: string, role: Role, glyph: string, meta: string}[]}
 */
export function summary(selection, repos, branch) {
  const pinned = new Map(repos.map((repo) => [repoID(repo), repo.default_branch]));
  return ordered(selection).map(({ id, role }) => ({
    id,
    role,
    glyph: GLYPHS[role],
    meta: role === "editing" ? editingMeta(branch) : referenceMeta(pinned.get(id)),
  }));
}

const editingMeta = (branch) => (branch ? "→ " + branch : "");

const referenceMeta = (pinned) => (pinned ? `→ ${pinned}, ${READ_ONLY}` : "→ " + READ_ONLY);
