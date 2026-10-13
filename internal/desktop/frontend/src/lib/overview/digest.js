import { age } from "../relative.js";

/** @typedef {(typeof import("../bridge/generated.js").CHROME_DEFAULTS)["sessionDigest"]} SessionDigest */

/** The So far tier's text and its age, or null when there is nothing to show.
 * @param {SessionDigest | undefined} digest @param {number} [now] */
export function digestMeta(digest, now = Date.now()) {
  const text = digest?.text?.trim() ?? "";
  if (!text) return null;
  const when = age(digest?.at ?? "", now);
  const meta = when && when !== "now" ? `${when} ago` : when;
  return { text, meta };
}
