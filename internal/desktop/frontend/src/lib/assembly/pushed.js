import { relative } from "../relative.js";

/**
 * @param {string|number|Date|undefined} at
 * @param {number} [now]
 */
export function pushed(at, now = Date.now()) {
  const since = relative(at ?? "", "prose", now);
  return since ? "pushed " + since : "";
}
