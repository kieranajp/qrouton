/** The selected tab remains visible even when it falls beyond capacity.
 * @template T @param {T[]} tabs
 * @param {number} selected @param {number} capacity
 * @returns {{shown: {tab: T, index: number}[], hidden: {tab: T, index: number}[]}} */
export function split(tabs, selected, capacity) {
  const all = tabs.map((tab, index) => ({ tab, index }));
  if (all.length <= capacity) return { shown: all, hidden: [] };
  const shown = capacity > 1 ? all.slice(0, capacity - 1) : [];
  if (!shown.some((entry) => entry.index === selected) && all[selected]) shown.push(all[selected]);
  const drawn = new Set(shown.map((entry) => entry.index));
  return { shown, hidden: all.filter((entry) => !drawn.has(entry.index)) };
}

/** @typedef {"waiting" | "failed" | "running" | "succeeded" | "idle"} TabStatus */

/** @type {TabStatus[]} */
const STATUS_PRIORITY = ["waiting", "failed", "running", "succeeded", "idle"];

/** @param {{status?: TabStatus}[]} tabs @returns {TabStatus | ""} */
export function dominantStatus(tabs) {
  return STATUS_PRIORITY.find((status) => tabs.some((tab) => tab.status === status)) ?? "";
}

/** @param {{badge?: string, label?: string}} tab */
export const tabLabel = ({ badge, label }) => (badge ? `${badge} ${label}` : (label ?? ""));

/** Maps a drop through the drawn row so hidden tabs do not skew its whole-strip position.
 * @param {{index: number}[]} shown drawn entries carrying whole-strip indices
 * @param {number} from dragged whole-strip index
 * @param {number} onto target whole-strip index @returns {number} resulting whole-strip index */
export function dropIndex(shown, from, onto) {
  const drawn = shown.map((entry) => entry.index);
  const target = drawn.indexOf(onto);
  if (target < 0 || !drawn.includes(from) || from === onto) return from;
  const rest = drawn.filter((index) => index !== from);
  const follows = target > 0 ? rest[target - 1] : -1;
  if (follows < 0) return 0;
  return follows < from ? follows + 1 : follows;
}
