import { relative } from "../relative.js";

/**
 * pushed is a repository row's meta line. A repository with no push behind it
 * says nothing rather than dating itself to the epoch.
 * @param {string|number|Date} at
 * @param {number} [now]
 */
export function pushed(at, now = Date.now()) {
  const since = relative(at, "prose", now);
  return since ? "pushed " + since : "";
}
