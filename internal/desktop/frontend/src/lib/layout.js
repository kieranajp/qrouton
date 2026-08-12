// Per-session layout, kept pure: node --test is the whole frontend harness.

const WIDTH_PREFIX = "qrouton.human-pane:";

export const widthKey = (slug) => WIDTH_PREFIX + slug;

/**
 * storedWidth is the width a session was left at. Zero means untouched, which
 * leaves the starting width the token's to own.
 * @param {(key: string) => string | null} read
 */
export const storedWidth = (read, slug) => Number(read(widthKey(slug))) || 0;

/** focusedIn is the tab id a session had up, held by id so a window docking
 * behind it cannot shift the selection the way an index would. */
export const focusedIn = (focus, slug) => focus[slug] ?? "";

/** focusIn records one session's focused tab and leaves every other session's. */
export const focusIn = (focus, slug, id) => ({ ...focus, [slug]: id });
