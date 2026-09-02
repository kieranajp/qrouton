// Applying a fetched ticket to the form, kept pure: node --test is the whole
// frontend harness.

/** @typedef {{name?: string, branchDescription?: string, description?: string, ticket?: string}} Draft */
/** @typedef {{url?: string, title?: string, body?: string, branchDescription?: string}} Result */

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
 * @param {{fetching?: (active: boolean) => void, loaded?: (fields: {name: string, branchDescription: string, description: string}) => void, failed?: (error: unknown) => void}} hooks
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

// Ticket results only fill empty fields when the requested URL remains current.
/**
 * @param {Draft} draft
 * @param {Result} result
 * @returns {{name: string, branchDescription: string, description: string}}
 */
export function fill(draft, result) {
  const kept = {
    name: draft.name ?? "",
    branchDescription: draft.branchDescription ?? "",
    description: draft.description ?? "",
  };
  if (!applies(draft, result)) return kept;
  return {
    name: trimmed(kept.name) ? kept.name : (result.title ?? ""),
    branchDescription: trimmed(kept.branchDescription)
      ? kept.branchDescription
      : (result.branchDescription ?? ""),
    description: trimmed(kept.description) ? kept.description : (result.body ?? ""),
  };
}
