import {
  ASSEMBLY_BEGIN,
  ASSEMBLY_CHECK,
  ASSEMBLY_CHECK_SLUG,
  ASSEMBLY_CREATE,
  ASSEMBLY_END,
  ASSEMBLY_FETCH,
  ASSEMBLY_PENDING,
  ASSEMBLY_PREFIXES,
  ASSEMBLY_PREVIEW,
  ASSEMBLY_RUNNERS,
  ORGS_LIST,
  PICKER_CANCEL,
  PICKER_CONFIRM,
  PICKER_ESCALATE,
  PICKER_LOAD,
  REPOSITORIES_CACHED,
  REPOSITORIES_REFRESH,
} from "../bridge/generated.js";
import { Call } from "../wails.js";

/**
 * @typedef {object} Draft
 * @property {string} name
 * @property {string} branchDescription
 * @property {string} description
 * @property {string} ticket
 * @property {string} prefix
 * @property {string} mode
 * @property {string} runner
 * @property {{id: string, role: string}[]} repos in the order they were picked
 */

/** prefixes is the branch-prefix vocabulary, which Go owns the only copy of. */
export const prefixes = () => Call.ByName(ASSEMBLY_PREFIXES);

/** runners are the agents with a resolved path, already filtered. */
export const runners = () => Call.ByName(ASSEMBLY_RUNNERS);

/** @param {Draft} draft */
export const check = (draft) => Call.ByName(ASSEMBLY_CHECK, draft);

/** checkSlug is the half that stats the disk, so it runs on advance.
 * @param {Draft} draft */
export const checkSlug = (draft) => Call.ByName(ASSEMBLY_CHECK_SLUG, draft);

/**
 * preview is the branch, which Go derives from the prefix and the slug alone.
 * @param {{name: string, branchDescription: string, ticket: string, entropy: string, prefix: string}} draft
 * @returns {Promise<string>}
 */
export const preview = (draft) => Call.ByName(ASSEMBLY_PREVIEW, draft);

/** @param {Draft} draft */
export const create = (draft) => Call.ByName(ASSEMBLY_CREATE, draft);

export const pending = () => Call.ByName(ASSEMBLY_PENDING);

export const begin = () => Call.ByName(ASSEMBLY_BEGIN);

/** @param {number} generation */
export const end = (generation) => Call.ByName(ASSEMBLY_END, generation);

/**
 * fetchTicket answers with the URL it asked about, which is what lets a result
 * for a URL the field has since moved off be dropped.
 * @param {string} url
 */
export const fetchTicket = async (url) => ({
  url,
  ...(await Call.ByName(ASSEMBLY_FETCH, url)),
});

export const cached = () => Call.ByName(REPOSITORIES_CACHED);

/** refresh answers with the generation its events will carry. */
export const refresh = () => Call.ByName(REPOSITORIES_REFRESH);

export const orgs = () => Call.ByName(ORGS_LIST);

/**
 * held is what the picker draws itself from: the branch anything added joins,
 * empty for a session with no repositories yet, and the rows already in it.
 * @param {string} slug
 * @returns {Promise<{branch: string, repos: {id: string, role: 'editing'|'reference', locked: boolean}[]}>}
 */
export const held = (slug) => Call.ByName(PICKER_LOAD, slug);

/** @param {string} slug */
export const escalate = (slug) => Call.ByName(PICKER_ESCALATE, slug);

/**
 * addRepos gives the session the picked repositories, takes up the held ones
 * named in upgrades, and gives the escalation waiting on it its answer.
 * @param {string} slug
 * @param {{repos: {id: string, role: string}[], upgrades: string[]}} answer
 */
export const addRepos = (slug, answer) => Call.ByName(PICKER_CONFIRM, slug, answer);

/** @param {string} slug */
export const cancelPicker = (slug) => Call.ByName(PICKER_CANCEL, slug);
