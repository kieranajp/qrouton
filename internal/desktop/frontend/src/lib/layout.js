// Per-session layout, kept pure: node --test is the whole frontend harness.

const WIDTH_PREFIX = "qrouton.human-pane:";
const SIDEBAR_WIDTH_KEY = "qrouton.sidebar";

export const widthKey = (slug) => WIDTH_PREFIX + slug;

export const sidebarWidthKey = () => SIDEBAR_WIDTH_KEY;

/** Zero leaves the starting width to CSS.
 * @param {(key: string) => string | null} read */
export const storedWidth = (read, slug) => Number(read(widthKey(slug))) || 0;

export const storedSidebarWidth = (read) => Number(read(SIDEBAR_WIDTH_KEY)) || 0;

/** A missing selection opens the first tab; a stale selection selects nothing.
 * @param {{id?: string}[]} tabs
 * @param {string} selected */
export function selectedTab(tabs, selected) {
  if (!selected) return tabs.length ? 0 : -1;
  return tabs.findIndex((tab) => tab.id === selected);
}

// Neither pane is worth having below these; the divider stops rather than
// letting one of them become a strip.
export const MIN_HUMAN = 320;
export const MIN_AGENT = 360;
export const MIN_SIDEBAR = 160;
export const MAX_SIDEBAR = 360;

export const sidebarWidth = (width) =>
  width ? Math.min(Math.max(width, MIN_SIDEBAR), MAX_SIDEBAR) : 0;

// Unmeasured panels impose no width limit.
export const roomFor = (panels, rail) =>
  panels ? Math.max(MIN_HUMAN, panels - rail - MIN_AGENT) : Infinity;

// Zero leaves the shell pane at its intrinsic starting width.
export const humanWidth = (width, room) =>
  width ? Math.min(Math.max(width, MIN_HUMAN), room) : 0;

// A page served from a custom scheme has an origin the webview may call opaque,
// where storage throws rather than coming back empty. Both of these are the
// whole reason the pane width is read and written through functions.

/** @param {string} key */
export const readStored = (key) => {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
};

/** @param {string} key */
export const writeStored = (key, value) => {
  try {
    if (value) localStorage.setItem(key, String(value));
    else localStorage.removeItem(key);
  } catch {}
};
