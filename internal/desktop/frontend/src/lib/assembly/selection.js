// Which repositories a session takes, and in what order.

import { GLYPHS, READ_ONLY } from "../roles.js";
import { repoID } from "./filter.js";

/** @typedef {'off'|'editing'|'reference'} Role */
/**
 * @typedef {object} Selection
 * @property {string[]} order the ids in the order they were picked, which is the ranking
 * @property {Record<string, Role>} roles for a held row, the role the session holds it in
 * @property {Record<string, string>} bases for a row cut from something other than the default branch, that branch
 * @property {string[]} locked the ids the session already holds
 * @property {string[]} upgrades the held ids to take up for editing
 */

const IN_SESSION = "in session";
const CUT_FROM = "from ";
const READING = "in session, read-only";
const TAKING_UP = "in session, taking it up to edit";

/** @type {Role[]} */
const OFFERS = ["off", "editing", "reference"];
/** @type {Role[]} */
const UPGRADE_OFFERS = ["reference", "editing"];
/** @type {Role[]} */
const NO_OFFERS = [];

/** Held rows retain their roles and the branch they were cut from, but stay out
 * of the new-selection order.
 * @param {{id: string, role: Role, base?: string}[]} [held]
 * @returns {Selection} */
export function seed(held = []) {
  /** @type {Record<string, Role>} */
  const roles = {};
  /** @type {Record<string, string>} */
  const bases = {};
  for (const row of held) {
    roles[row.id] = row.role;
    if (row.base) bases[row.id] = row.base;
  }
  return { order: [], roles, bases, locked: held.map((row) => row.id), upgrades: [] };
}

/** @returns {Role} */
export const roleOf = (selection, id) =>
  isUpgrading(selection, id) ? "editing" : (selection.roles[id] ?? "off");

export const isLocked = (selection, id) => selection.locked.includes(id);

export const isUpgrading = (selection, id) => selection.upgrades.includes(id);

/** baseOf is the branch a row's work is cut from, empty for the default one. */
export const baseOf = (selection, id) => selection.bases?.[id] ?? "";

/** Held editing repositories cannot be dropped or demoted from the picker.
 * @returns {Role[]} */
export function roleOffers(selection, id) {
  if (!isLocked(selection, id)) return OFFERS;
  return selection.roles[id] === "reference" ? UPGRADE_OFFERS : NO_OFFERS;
}

/**
 * @param {Selection} selection
 * @param {string} id
 * @param {string} pushed
 * @param {string} [defaultBranch]
 */
export function rowMeta(selection, id, pushed, defaultBranch = "") {
  return [pushed, heldNote(selection, id), baseNote(selection, id, defaultBranch)]
    .filter(Boolean)
    .join(" · ");
}

function baseNote(selection, id, defaultBranch) {
  if (!isLocked(selection, id)) return "";
  const base = baseOf(selection, id);
  return base && base !== defaultBranch ? CUT_FROM + base : "";
}

function heldNote(selection, id) {
  if (!isLocked(selection, id)) return "";
  if (isUpgrading(selection, id)) return TAKING_UP;
  return selection.roles[id] === "reference" ? READING : IN_SESSION;
}

/** Demotion preserves selection rank; turning a repository off discards it.
 * The base a row is cut from survives both.
 * @returns {Selection} */
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

/** @returns {Selection} */
export function setBase(selection, id, branch) {
  const bases = { ...selection.bases };
  if (branch) bases[id] = branch;
  else delete bases[id];
  return { ...selection, bases };
}

/** A pending upgrade retains its on-disk role until confirmation.
 * @returns {Selection} */
function takeUp(selection, id, role) {
  if (!roleOffers(selection, id).includes(role)) return selection;
  const upgrades = selection.upgrades.filter((seen) => seen !== id);
  return { ...selection, upgrades: role === "editing" ? [...upgrades, id] : upgrades };
}

/** Held rows survive a list that omits them and keep the manifest's spelling. A
 * request names repositories the cache had not seen and may spell one differently
 * than GitHub does, so a picked row takes the list's spelling.
 * @param {Selection} selection @param {string[]} ids @returns {Selection} */
export function reconcile(selection, ids) {
  const canonical = new Map(ids.map((id) => [id.toLowerCase(), id]));
  const keep = (id) => canonical.has(id.toLowerCase()) || isLocked(selection, id);
  const spell = (id) => (isLocked(selection, id) ? id : (canonical.get(id.toLowerCase()) ?? id));
  /** @type {Record<string, Role>} */
  const roles = {};
  for (const [id, role] of Object.entries(selection.roles)) {
    if (keep(id)) roles[spell(id)] = role;
  }
  /** @type {Record<string, string>} */
  const bases = {};
  for (const [id, base] of Object.entries(selection.bases ?? {})) {
    if (keep(id)) bases[spell(id)] = base;
  }
  return { ...selection, roles, bases, order: selection.order.filter(keep).map(spell) };
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
 * @returns {{id: string, role: Role, base: string}[]}
 */
export const ordered = (selection) =>
  selection.order.map((id) => ({ id, role: selection.roles[id], base: baseOf(selection, id) }));

/** upgrading is what Go takes up for editing, which it finds in the manifest. */
export const upgrading = (selection) => [...selection.upgrades];

/** A row already held routes through setRole to takeUp, so an upgrade needs no branch here.
 * @param {Selection} selection
 * @param {{id: string, role: Role}[]} [requested]
 * @returns {Selection} */
export function preselect(selection, requested = []) {
  let next = selection;
  for (const row of requested) {
    if (!row?.id) continue;
    next = setRole(next, row.id, row.role);
  }
  return next;
}

/** Pending upgrades lead because their held rows may be filtered out.
 * @param {Selection} selection
 * @param {{org: string, name: string, default_branch?: string}[]} repos
 * @param {string} branch @returns {{id: string, role: Role, glyph: string, meta: string}[]} */
export function summary(selection, repos, branch) {
  const pinned = new Map(repos.map((repo) => [repoID(repo), repo.default_branch]));
  const taken = selection.upgrades.map((id) => ({ id, role: roleOf(selection, id) }));
  return [...taken, ...ordered(selection)].map(({ id, role }) => ({
    id,
    role,
    glyph: GLYPHS[role],
    meta:
      role === "editing"
        ? editingMeta(branch)
        : referenceMeta(baseOf(selection, id) || pinned.get(id)),
  }));
}

const editingMeta = (branch) => (branch ? "→ " + branch : "");

const referenceMeta = (pinned) => (pinned ? `→ ${pinned}, ${READ_ONLY}` : "→ " + READ_ONLY);
