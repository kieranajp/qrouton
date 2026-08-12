// Applying a fetched ticket to the form, kept pure: node --test is the whole
// frontend harness.

/** @typedef {{name?: string, description?: string, ticket?: string}} Draft */
/** @typedef {{url?: string, title?: string, body?: string}} Result */

const trimmed = (text) => (text ?? "").trim();

/**
 * fill applies a fetched ticket to the two fields it may touch. A result for a
 * URL the field has since moved off is dropped, and a fill only ever lands in an
 * empty field.
 * @param {Draft} draft
 * @param {Result} result
 * @returns {{name: string, description: string}}
 */
export function fill(draft, result) {
  const kept = { name: draft.name ?? "", description: draft.description ?? "" };
  const url = trimmed(result?.url);
  if (!url || url !== trimmed(draft.ticket)) return kept;
  return {
    name: trimmed(kept.name) ? kept.name : (result.title ?? ""),
    description: trimmed(kept.description) ? kept.description : (result.body ?? ""),
  };
}
