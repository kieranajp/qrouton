/**
 * split divides the tabs into the ones the strip draws and the ones its menu
 * lists, each carrying its index in the whole strip so a click still names the
 * window it meant. The selected tab is always drawn.
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

/**
 * tabLabel is a tab's whole text: its artifact badge, when it has one, then the
 * title. The strip draws the two apart so the badge can carry its own colour;
 * a tooltip and a menu row want them back together.
 * @param {{badge?: string, label?: string}} tab
 */
export const tabLabel = ({ badge, label }) => (badge ? `${badge} ${label}` : (label ?? ""));

/**
 * dropIndex places a tab dropped onto another in the whole strip. The strip may
 * be drawing only some of the tabs, so the drop is read against the drawn row:
 * the dragged tab lands immediately after the drawn tab it comes to follow, or
 * at the front when it follows none. Taking the drop target's place in the
 * drawn row for a place in the whole strip would land the tab among the ones in
 * the overflow menu.
 * @param {{index: number}[]} shown drawn entries, each carrying its index in the whole strip
 * @param {number} from the dragged tab's index in the whole strip
 * @param {number} onto the index of the tab it was dropped on
 * @returns {number} the dragged tab's index in the whole strip after the move
 */
export function dropIndex(shown, from, onto) {
  const drawn = shown.map((entry) => entry.index);
  const target = drawn.indexOf(onto);
  if (target < 0 || !drawn.includes(from) || from === onto) return from;
  const rest = drawn.filter((index) => index !== from);
  const follows = target > 0 ? rest[target - 1] : -1;
  if (follows < 0) return 0;
  return follows < from ? follows + 1 : follows;
}
