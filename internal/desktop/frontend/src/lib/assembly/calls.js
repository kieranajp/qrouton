import { Call } from "../wails.js";

const ASSEMBLY_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Assembly";
const REPOSITORIES_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Repositories";
const ORGS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Orgs";
const PICKER_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Picker";

/**
 * @typedef {object} Draft
 * @property {string} name
 * @property {string} description
 * @property {string} ticket
 * @property {string} prefix
 * @property {string} mode
 * @property {string} runner
 * @property {{id: string, role: string}[]} repos in the order they were picked
 */

/** prefixes is the branch-prefix vocabulary, which Go owns the only copy of. */
export const prefixes = () => Call.ByName(ASSEMBLY_SERVICE + ".Prefixes");

/** runners are the agents with a resolved path, already filtered. */
export const runners = () => Call.ByName(ASSEMBLY_SERVICE + ".Runners");

/** @param {Draft} draft */
export const check = (draft) => Call.ByName(ASSEMBLY_SERVICE + ".Check", draft);

/** checkSlug is the half that stats the disk, so it runs on advance.
 * @param {Draft} draft */
export const checkSlug = (draft) => Call.ByName(ASSEMBLY_SERVICE + ".CheckSlug", draft);

/** @param {Draft} draft */
export const preview = (draft) => Call.ByName(ASSEMBLY_SERVICE + ".Preview", draft);

/** @param {Draft} draft */
export const create = (draft) => Call.ByName(ASSEMBLY_SERVICE + ".Create", draft);

/**
 * fetchTicket answers with the URL it asked about, which is what lets a result
 * for a URL the field has since moved off be dropped.
 * @param {string} url
 */
export const fetchTicket = async (url) => ({
  url,
  ...(await Call.ByName(ASSEMBLY_SERVICE + ".Fetch", url)),
});

export const cached = () => Call.ByName(REPOSITORIES_SERVICE + ".Cached");

/** refresh answers with the generation its events will carry. */
export const refresh = () => Call.ByName(REPOSITORIES_SERVICE + ".Refresh");

export const orgs = () => Call.ByName(ORGS_SERVICE + ".List");

/**
 * held is what the picker draws itself from: the branch anything added joins,
 * empty for a session with no repositories yet, and the rows already in it.
 * @param {string} slug
 * @returns {Promise<{branch: string, repos: {id: string, role: 'editing'|'reference', locked: boolean}[]}>}
 */
export const held = (slug) => Call.ByName(PICKER_SERVICE + ".Load", slug);

/**
 * addRepos gives the session the picked repositories, and the escalation waiting
 * on it its answer. Only repos is read; the rest of the draft is the session's.
 * @param {string} slug
 * @param {{repos: {id: string, role: string}[]}} draft
 */
export const addRepos = (slug, draft) => Call.ByName(PICKER_SERVICE + ".Confirm", slug, draft);

/** @param {string} slug */
export const cancelPicker = (slug) => Call.ByName(PICKER_SERVICE + ".Cancel", slug);
