// Per-session layout, kept pure: node --test is the whole frontend harness.

const WIDTH_PREFIX = "qrouton.human-pane:";
const SIDEBAR_WIDTH_KEY = "qrouton.sidebar";

export const widthKey = (slug) => WIDTH_PREFIX + slug;

export const sidebarWidthKey = () => SIDEBAR_WIDTH_KEY;

/**
 * storedWidth is the width a session was left at. Zero means untouched, which
 * leaves the starting width the token's to own.
 * @param {(key: string) => string | null} read
 */
export const storedWidth = (read, slug) => Number(read(widthKey(slug))) || 0;

export const storedSidebarWidth = (read) => Number(read(SIDEBAR_WIDTH_KEY)) || 0;

export const selectedIn = (selection, slug) => selection[slug] ?? "";

export const selectIn = (selection, slug, id) => ({ ...selection, [slug]: id });

export const focusGenerationIn = (generations, id) => generations[id]?.generation ?? 0;

export const focusPendingIn = (generations, id) => generations[id]?.pending ?? false;

export const focusTerminal = (generations, id) => ({
  ...generations,
  [id]: { generation: focusGenerationIn(generations, id) + 1, pending: true },
});

export function consumeTerminalFocus(generations, id, generation) {
  const current = generations[id];
  if (!current || current.generation !== generation || !current.pending) return generations;
  return { ...generations, [id]: { generation, pending: false } };
}

// Neither pane is worth having below these; the divider stops rather than
// letting one of them become a strip.
export const MIN_HUMAN = 320;
export const MIN_AGENT = 360;
export const MIN_SIDEBAR = 160;
export const MAX_SIDEBAR = 360;

export const sidebarWidth = (width) =>
  width ? Math.min(Math.max(width, MIN_SIDEBAR), MAX_SIDEBAR) : 0;

/**
 * roomFor is the widest the shell pane may be drawn: whatever is left once the
 * rail and the agent's minimum are taken out. Before the panels have been
 * measured there is no known limit, so nothing is clamped yet.
 */
export const roomFor = (panels, rail) =>
  panels ? Math.max(MIN_HUMAN, panels - rail - MIN_AGENT) : Infinity;

/**
 * humanWidth is the width the shell pane actually gets: the stored or dragged
 * one, never below its own minimum and never past the room there is. Zero means
 * untouched, which leaves the starting width to the pane itself.
 */
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
