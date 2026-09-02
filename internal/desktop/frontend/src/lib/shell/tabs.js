// The selected tab remains visible even when it falls beyond capacity.
/**
 * @template T
 * @param {T[]} tabs
 * @param {number} selected
 * @param {number} capacity
 * @returns {{shown: {tab: T, index: number}[], hidden: {tab: T, index: number}[]}}
 */
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
