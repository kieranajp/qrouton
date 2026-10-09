// Keyboard shortcuts the page handles itself.

import { UI_SCALE_ACTIONS } from "./bridge/generated.js";

// NUMBERED is how many rail rows get a shortcut. Past that the rows are
// click-only: there is no second modifier worth teaching and no digit left.
export const NUMBERED = 9;

/** Numbered rail shortcuts require Command alone to avoid terminal bindings.
 * @param {{key?: string, metaKey?: boolean, ctrlKey?: boolean, altKey?: boolean, shiftKey?: boolean}} event */
export function position(event) {
  if (!event?.metaKey || event.ctrlKey || event.altKey || event.shiftKey) return 0;
  const digit = Number(event.key);
  if (!Number.isInteger(digit) || digit < 1 || digit > NUMBERED) return 0;
  return digit;
}

/** @param {any[]} rows */
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

/** Command on macOS and Control on Linux; Control on macOS stays terminal input.
 * @param {{key?: string, metaKey?: boolean, ctrlKey?: boolean, altKey?: boolean, shiftKey?: boolean}} event
 * @param {boolean} linux */
export function scaleAction(event, linux) {
  if (!event || event.altKey) return "";
  const held = linux ? event.ctrlKey && !event.metaKey : event.metaKey && !event.ctrlKey;
  if (!held) return "";
  if (event.key === "=" || event.key === "+") return UI_SCALE_ACTIONS.IN;
  if (event.key === "-") return UI_SCALE_ACTIONS.OUT;
  if (event.key === "0") return UI_SCALE_ACTIONS.RESET;
  return "";
}
