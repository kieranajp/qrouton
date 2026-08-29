// Switching sessions by keyboard. Pure so node --test can reach it, which is the
// whole frontend harness.

// NUMBERED is how many rail rows get a shortcut. Past that the rows are
// click-only: there is no second modifier worth teaching and no digit left.
export const NUMBERED = 9;

/**
 * position is the rail row a keystroke names, counting from one, or zero for a
 * keystroke that names none. Command alone, so it does not collide with the
 * terminal's own Control and Option bindings.
 * @param {{key?: string, metaKey?: boolean, ctrlKey?: boolean, altKey?: boolean, shiftKey?: boolean}} event
 */
export function position(event) {
  if (!event?.metaKey || event.ctrlKey || event.altKey || event.shiftKey) return 0;
  const digit = Number(event.key);
  if (!Number.isInteger(digit) || digit < 1 || digit > NUMBERED) return 0;
  return digit;
}

/**
 * rowAt is the rail row a keystroke names, and undefined for a keystroke that
 * names none or one naming a row that is not there.
 * @param {any[]} rows
 */
export const rowAt = (rows, event) => rows?.[position(event) - 1];

/** shortcut is the glyph a rail row wears in place of its initials. */
export const shortcut = (index) => (index < NUMBERED ? "⌘" + (index + 1) : "");

/**
 * opensSettings is Command on macOS and Control elsewhere, with a comma.
 * @param {{key?: string, metaKey?: boolean, ctrlKey?: boolean, altKey?: boolean, shiftKey?: boolean}} event
 */
export function opensSettings(event) {
  if (!event || event.altKey || event.shiftKey) return false;
  return Boolean(event.metaKey) !== Boolean(event.ctrlKey) && event.key === ",";
}
