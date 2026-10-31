import { relative } from "../relative.js";

/** @typedef {(typeof import("../bridge/generated.js").CHROME_DEFAULTS)["sessionDigest"]} SessionDigest */

/** The So far tier's text and its age, or null when there is nothing to show.
 * @param {SessionDigest | undefined} digest @param {number} [now] */
export function digestMeta(digest, now = Date.now()) {
  const text = digest?.text?.trim() ?? "";
  if (!text) return null;
  const meta = relative(digest?.at ?? "", "ago", now);
  return { text, meta };
}
