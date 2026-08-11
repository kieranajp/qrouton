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
