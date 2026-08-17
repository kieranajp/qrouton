// What a first-run Save response means for the screen, kept pure: node --test is
// the whole frontend harness.

import { fieldError } from "../settings/errors.js";

/**
 * firstRunOutcome names the field and the footer a refusal names, and says
 * nothing at all on success. There is no close: the gate is Go's, dropped by the
 * chrome payload or by the window going with the process.
 * @param {{relaunching?: boolean} | undefined} result
 * @param {any} err
 * @returns {{fields: Record<string, string>, status: string}}
 */
export function firstRunOutcome(result, err) {
  if (!err) return { fields: {}, status: "" };
  const found = fieldError(err);
  return {
    fields: found ? { [found.field]: found.message } : {},
    status: found ? found.message : String(err?.message ?? err ?? ""),
  };
}
