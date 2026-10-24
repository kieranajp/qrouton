// What a Save refusal means for the panel's fields.

import { SETTINGS_FIELDS } from "../bridge/generated.js";
import { fieldError as parse } from "../fields.js";

const msgLoadFailed = "Settings could not be read:";

const STICKER_ROW = `${SETTINGS_FIELDS.STICKERS}\\.\\d+`;

/** Unscoped failures return null instead of guessing a field.
 * @param {any} err
 * @returns {{field: string, message: string} | null} */
export const fieldError = (err) => parse(err, SETTINGS_FIELDS, [STICKER_ROW]);

/** @param {any} err */
export function loadFailure(err) {
  const found = fieldError(err);
  return `${msgLoadFailed} ${found ? found.message : String(err?.message ?? err ?? "")}`;
}

/** Failures omit restartRequired so an existing restart banner remains unchanged.
 * @param {{restartRequired?: boolean} | undefined} result
 * @param {any} err
 * @returns {{close: boolean, restartRequired?: boolean, fields: Record<string, string>, status: string}} */
export function saveOutcome(result, err) {
  if (err) {
    const found = fieldError(err);
    return {
      close: false,
      fields: found ? { [found.field]: found.message } : {},
      status: found ? found.message : String(err?.message ?? err ?? ""),
    };
  }
  const restartRequired = !!result?.restartRequired;
  return { close: !restartRequired, restartRequired, fields: {}, status: "" };
}
