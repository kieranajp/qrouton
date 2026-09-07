// Branch lists for the base menu, keyed by repository.

/** @typedef {'idle'|'loading'|'ready'|'failed'} State */
/**
 * @typedef {object} Entry
 * @property {State} state
 * @property {string[]} branches the default branch first, as Go orders them
 * @property {string} default
 * @property {string} [error]
 */

/** @type {Entry} */
const UNASKED = { state: "idle", branches: [], default: "" };

/** @returns {Record<string, Entry>} */
export const unasked = () => ({});

/**
 * @param {Record<string, Entry>} lists
 * @param {string} id
 * @returns {Entry}
 */
export const entry = (lists, id) => lists[id] ?? UNASKED;

/** A listing that failed is worth asking for again; one held or in flight is not. */
export function wanted(lists, id) {
  const { state } = entry(lists, id);
  return state === "idle" || state === "failed";
}

/** requested keeps whatever the entry already held, so re-opening a menu draws
 * the last answer rather than an empty list.
 * @returns {Record<string, Entry>} */
export function requested(lists, id) {
  if (!wanted(lists, id)) return lists;
  return { ...lists, [id]: { ...entry(lists, id), state: "loading", error: undefined } };
}

/** Go answers a failure as a populated error beside the fallback it chose, so
 * the menu always has something to offer.
 * @param {{branches?: string[], default?: string, error?: string}} answer
 * @returns {Record<string, Entry>} */
export function settled(lists, id, answer) {
  return {
    ...lists,
    [id]: {
      state: answer?.error ? "failed" : "ready",
      branches: answer?.branches ?? [],
      default: answer?.default ?? "",
      error: answer?.error || undefined,
    },
  };
}
