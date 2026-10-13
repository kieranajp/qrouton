/** @typedef {(typeof import("../bridge/generated.js").CHROME_DEFAULTS)["pullRequests"]} PullRequestList */

export const SIGNED_OUT = "GitHub is not signed in";

/** What the Pull requests tier draws: rows, the signed-out line, or nothing.
 * @param {PullRequestList | undefined} list
 * @returns {{rows: {key: string, title: string, number: string, url: string}[], note: string} | null} */
export function pullRows(list) {
  if (list?.signedOut) return { rows: [], note: SIGNED_OUT };
  const items = list?.items ?? [];
  if (!items.length) return null;
  const rows = items.map((pr) => ({
    key: `${pr.repo}#${pr.number}`,
    title: pr.title,
    number: `#${pr.number}`,
    url: pr.url,
  }));
  return { rows, note: "" };
}
