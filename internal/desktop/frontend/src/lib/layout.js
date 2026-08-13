// Per-session layout, kept pure: node --test is the whole frontend harness.

const WIDTH_PREFIX = "qrouton.human-pane:";

export const widthKey = (slug) => WIDTH_PREFIX + slug;

/**
 * storedWidth is the width a session was left at. Zero means untouched, which
 * leaves the starting width the token's to own.
 * @param {(key: string) => string | null} read
 */
export const storedWidth = (read, slug) => Number(read(widthKey(slug))) || 0;

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
