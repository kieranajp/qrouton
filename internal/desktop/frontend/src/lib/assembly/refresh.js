// The repository refresh as the page sees it, kept pure: node --test is the whole
// frontend harness.

/** @typedef {'fetching'|'updated'|'failed'} OwnerStatus */
const STATES = { started: "fetching", succeeded: "updated", failed: "failed" };

/**
 * @typedef {object} Refresh
 * @property {number} generation the newest one seen
 * @property {boolean} active
 * @property {Record<string, {status: OwnerStatus, error?: string}>} owners
 * @property {{org: string, name: string}[]} repos
 */
/**
 * @typedef {object} Event
 * @property {number} generation
 * @property {string} state
 * @property {string} [owner]
 * @property {{org: string, name: string}[]} [repos]
 * @property {string} [error]
 */

/** @returns {Refresh} */
export const idle = (repos = []) => ({ generation: 0, active: false, owners: {}, repos });

// Events can precede their initiating call's response, but older generations are stale.
/**
 * @param {Refresh} refresh
 * @param {Event} event
 * @returns {Refresh}
 */
export function apply(refresh, event) {
  const generation = event?.generation ?? 0;
  if (generation < refresh.generation) return refresh;
  const next = {
    ...refresh,
    generation,
    repos: Array.isArray(event.repos) ? event.repos : refresh.repos,
  };
  if (event.state === "complete") return { ...next, active: false };
  const status = STATES[event.state];
  if (!status) return next;
  return {
    ...next,
    active: true,
    owners: { ...next.owners, [event.owner]: { status, error: event.error || undefined } },
  };
}

/** failedOwners are the owners whose last attempt errored, which is what a retry refetches. */
export const failedOwners = (refresh) =>
  Object.keys(refresh.owners).filter((owner) => refresh.owners[owner].status === "failed");
