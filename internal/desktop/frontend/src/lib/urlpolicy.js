// Browser.OpenURL hands the OS whatever scheme reaches it; this predicate is
// the only filter standing between a link and that handoff.

/** @param {string} url */
export function isAllowedURL(url) {
  try {
    return /^https?:$/.test(new URL(url).protocol);
  } catch {
    return false;
  }
}
