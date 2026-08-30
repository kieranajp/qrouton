// What a Save refusal means for the panel's fields, kept pure: node --test is
// the whole frontend harness.

const msgLoadFailed = "Settings could not be read:";

/**
 * fieldError splits a `field: message` refusal into both parts, so the
 * footer and the named field can agree. A message with no leading field name
 * — a plain disk-write failure — answers null rather than a wrong guess.
 * @param {any} err
 * @returns {{field: string, message: string} | null}
 */
export function fieldError(err) {
  const text = String(err?.message ?? err ?? "");
  const found = text.match(/^([a-z]+): (.*)$/s);
  return found ? { field: found[1], message: found[2] } : null;
}

/**
 * loadFailure is what the panel says when the config could not be read at all:
 * every field is empty for a reason nothing else on screen gives.
 * @param {any} err
 */
export function loadFailure(err) {
  const found = fieldError(err);
  return `${msgLoadFailed} ${found ? found.message : String(err?.message ?? err ?? "")}`;
}

/**
 * saveOutcome is what a Save response means for the panel: close on a save
 * that touched nothing needing a restart, stay open behind a banner when one
 * is needed, or stay open naming the field and footer a refusal names.
 * restartRequired is omitted on a refusal, leaving the banner as it was.
 * @param {{restartRequired?: boolean} | undefined} result
 * @param {any} err
 * @returns {{close: boolean, restartRequired?: boolean, fields: Record<string, string>, status: string}}
 */
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
