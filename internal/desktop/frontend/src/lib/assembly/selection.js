// Which repositories a session takes and in what order, kept pure: node --test
// is the whole frontend harness.

import { GLYPHS, READ_ONLY } from "../roles.js";
import { repoID } from "./filter.js";

/** @typedef {'off'|'editing'|'reference'} Role */
/**
 * @typedef {object} Selection
 * @property {string[]} order the ids in the order they were picked, which is the ranking
 * @property {Record<string, Role>} roles for a held row, the role the session holds it in
 * @property {string[]} locked the ids the session already holds
 * @property {string[]} upgrades the held ids to take up for editing
 */

const IN_SESSION = "in session";
const READING = "in session, read-only";
const TAKING_UP = "in session, taking it up to edit";

/** @type {Role[]} */
const OFFERS = ["off", "editing", "reference"];
/** @type {Role[]} */
const UPGRADE_OFFERS = ["reference", "editing"];
/** @type {Role[]} */
const NO_OFFERS = [];

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
  return { order: [], roles, locked: held.map((row) => row.id), upgrades: [] };
}

/** @returns {Role} */
export const roleOf = (selection, id) =>
  isUpgrading(selection, id) ? "editing" : (selection.roles[id] ?? "off");

export const isLocked = (selection, id) => selection.locked.includes(id);

export const isUpgrading = (selection, id) => selection.upgrades.includes(id);

/**
 * roleOffers is what one row's toggle will answer to. A held editing repository
 * answers to nothing: its worktree can carry commits and uncommitted work, so
 * dropping or demoting it is not a thing to do from a picker.
 * @returns {Role[]}
 */
export function roleOffers(selection, id) {
  if (!isLocked(selection, id)) return OFFERS;
  return selection.roles[id] === "reference" ? UPGRADE_OFFERS : NO_OFFERS;
}

/**
 * rowMeta is the line beside a repository's name. A held row says so, and one
 * being taken up says that before it happens: the toggle beside it has answered.
 * @param {Selection} selection
 * @param {string} id
 * @param {string} pushed
 */
export function rowMeta(selection, id, pushed) {
  return [pushed, heldNote(selection, id)].filter(Boolean).join(" · ");
}

function heldNote(selection, id) {
  if (!isLocked(selection, id)) return "";
  if (isUpgrading(selection, id)) return TAKING_UP;
  return selection.roles[id] === "reference" ? READING : IN_SESSION;
}

/**
 * setRole records one row's role. A repository keeps the rank it was first given,
 * so demoting it to reference holds its place while turning it off and back sends
 * it last.
 * @returns {Selection}
 */
export function setRole(selection, id, role) {
  if (isLocked(selection, id)) return takeUp(selection, id, role);
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
 * takeUp marks a held reference row to be checked out for editing, or unmarks it.
 * The held role stays in roles, so the row can go back to what the session still
 * has on disk right up until confirm.
 * @returns {Selection}
 */
function takeUp(selection, id, role) {
  if (!roleOffers(selection, id).includes(role)) return selection;
  const upgrades = selection.upgrades.filter((seen) => seen !== id);
  return { ...selection, upgrades: role === "editing" ? [...upgrades, id] : upgrades };
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
  for (const id of Object.keys(selection.roles)) {
    const role = roleOf(selection, id);
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

/** upgrading is what Go takes up for editing, which it finds in the manifest. */
export const upgrading = (selection) => [...selection.upgrades];

/**
 * summary is the selected chips. An editing chip names the branch it joins, and
 * says nothing while there is no branch to name. A row being taken up leads: it
 * is the one answer whose row a search can scroll out of sight.
 * @param {Selection} selection
 * @param {{org: string, name: string, default_branch?: string}[]} repos
 * @param {string} branch
 * @returns {{id: string, role: Role, glyph: string, meta: string}[]}
 */
export function summary(selection, repos, branch) {
  const pinned = new Map(repos.map((repo) => [repoID(repo), repo.default_branch]));
  const taken = selection.upgrades.map((id) => ({ id, role: roleOf(selection, id) }));
  return [...taken, ...ordered(selection)].map(({ id, role }) => ({
    id,
    role,
    glyph: GLYPHS[role],
    meta: role === "editing" ? editingMeta(branch) : referenceMeta(pinned.get(id)),
  }));
}

const editingMeta = (branch) => (branch ? "→ " + branch : "");

const referenceMeta = (pinned) => (pinned ? `→ ${pinned}, ${READ_ONLY}` : "→ " + READ_ONLY);
