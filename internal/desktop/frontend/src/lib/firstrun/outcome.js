// What a first-run Save response means for the screen, kept pure: node --test is
// the whole frontend harness.

import { fieldError } from "../settings/errors.js";

/** Go owns dismissal of the first-run gate after a successful outcome.
 * @param {{relaunching?: boolean} | undefined} result
 * @param {any} err
 * @returns {{fields: Record<string, string>, status: string}} */
export function firstRunOutcome(result, err) {
  if (!err) return { fields: {}, status: "" };
  const found = fieldError(err);
  return {
    fields: found ? { [found.field]: found.message } : {},
    status: found ? found.message : String(err?.message ?? err ?? ""),
  };
}
