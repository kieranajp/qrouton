// Applying a fetched ticket to the form, kept pure: node --test is the whole
// frontend harness.

/** @typedef {{name?: string, description?: string, ticket?: string}} Draft */
/** @typedef {{url?: string, title?: string, body?: string}} Result */

const trimmed = (text) => (text ?? "").trim();

/**
 * @param {Draft} draft
 * @param {Result} result
 */
export const applies = (draft, result) => {
  const url = trimmed(result?.url);
  return !!url && url === trimmed(draft?.ticket);
};

export const claimSeed = (current, external, claimed = "") => {
  const ticket = trimmed(external);
  return ticket && !trimmed(current) && !trimmed(claimed) ? ticket : "";
};

/**
 * @param {Draft} draft
 * @param {(url: string) => Promise<Result>} fetchTicket
 * @param {{fetching?: (active: boolean) => void,
 *   loaded?: (fields: {name: string, description: string}) => void,
 *   failed?: (error: unknown) => void}} hooks
 */
export function loader(draft, fetchTicket, hooks = {}) {
  let fetching = false;
  let seeded = "";

  async function load() {
    const url = trimmed(draft.ticket);
    if (!url || fetching) return false;
    fetching = true;
    hooks.fetching?.(true);
    try {
      const result = await fetchTicket(url);
      if (!applies(draft, result)) return false;
      hooks.loaded?.(fill(draft, result));
      return true;
    } catch (err) {
      hooks.failed?.(err);
      return false;
    } finally {
      fetching = false;
      hooks.fetching?.(false);
    }
  }

  function seed(external) {
    const claimed = claimSeed(draft.ticket, external, seeded);
    if (!claimed) return false;
    seeded = claimed;
    draft.ticket = claimed;
    return load();
  }

  return { load, seed };
}

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
  if (!applies(draft, result)) return kept;
  return {
    name: trimmed(kept.name) ? kept.name : (result.title ?? ""),
    description: trimmed(kept.description) ? kept.description : (result.body ?? ""),
  };
}
