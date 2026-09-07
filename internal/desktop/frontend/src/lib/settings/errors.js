// What a Save refusal means for the panel's fields, kept pure: node --test is
// the whole frontend harness.

const msgLoadFailed = "Settings could not be read:";

/** Unscoped failures return null instead of guessing a field.
 * @param {any} err
 * @returns {{field: string, message: string} | null} */
export function fieldError(err) {
  const text = String(err?.message ?? err ?? "");
  const found = text.match(/^([a-z]+): (.*)$/s);
  return found ? { field: found[1], message: found[2] } : null;
}

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
