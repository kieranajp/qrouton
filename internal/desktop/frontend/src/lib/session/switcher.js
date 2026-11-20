import { DASHBOARD_TITLE } from "../dashboard/dashboard.js";

// Slugs are [a-z0-9-], so a colon keeps this key clear of every session.
export const DASHBOARD_KEY = ":dashboard";
const DASHBOARD = Object.freeze({ dashboard: true, slug: DASHBOARD_KEY, name: DASHBOARD_TITLE, repos: [] });

/** The switcher's key for what is on screen: the session, or the dashboard when none is.
 * @param {string} slug */
export const shownKey = (slug) => slug || DASHBOARD_KEY;

function score(token, value) {
  value = String(value ?? "").toLowerCase();
  if (value === token) return 4000;
  if (value.startsWith(token)) return 3000;
  if (value.includes(token)) return 2000;
  let last = -1;
  let bonus = 0;
  for (const letter of token) {
    const at = value.indexOf(letter, last + 1);
    if (at < 0) return 0;
    bonus += (at === last + 1 ? 8 : 0) + (at === 0 || /[\s/_-]/.test(value[at - 1]) ? 8 : 0) - Math.min(8, at - last - 1);
    last = at;
  }
  return 1000 + Math.max(-200, Math.min(200, bonus));
}

export function matchSessions(rows = [], query = "") {
  const tokens = query.trim().toLowerCase().split(/\s+/).filter(Boolean);
  const names = new Map();
  for (const row of rows) names.set(row.name, (names.get(row.name) ?? 0) + 1);
  return [DASHBOARD, ...rows].map((row, order) => {
    const fields = row.dashboard ? [row.name] : [row.name, row.slug, ...(row.repos ?? []).map((repo) => repo.name)];
    let quality = 0;
    let priority = 0;
    const context = new Set(names.get(row.name) > 1 ? [row.slug] : []);
    for (const token of tokens) {
      const matches = fields.map((value, index) => ({ value, index, score: score(token, value) }));
      matches.sort((a, b) => b.score - a.score || a.index - b.index);
      const best = matches[0];
      if (!best.score) return null;
      quality += best.score;
      priority += Math.min(2, best.index);
      if (best.index === 1 || best.index > 2) context.add(best.value);
    }
    return { row, quality, priority, order, context: [...context].filter(Boolean).join(" · ") };
  }).filter((match) => match !== null).sort((a, b) => b.quality - a.quality || a.priority - b.priority || a.order - b.order);
}

const sessions = (matches) => matches.filter(({ row }) => !row.dashboard);

/** The Dashboard entry is passed over, so opening and pressing Enter still switches session. */
export const initialSelection = (matches, current) =>
  (sessions(matches).find(({ row }) => row.slug !== current) ?? sessions(matches)[0] ?? matches[0])?.row.slug ?? "";

/** A query picks its best match; an empty one leaves the Dashboard entry for the user to choose. */
export const preferredSelection = (matches, query = "") =>
  (query.trim() ? matches[0] : sessions(matches)[0] ?? matches[0])?.row.slug ?? "";

export const reconcileSelection = (matches, slug, query = "") =>
  matches.some(({ row }) => row.slug === slug) ? slug : preferredSelection(matches, query);
