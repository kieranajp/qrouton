import { PULL_REQUEST_STATES } from "../bridge/generated.js";

/** @typedef {(typeof import("../bridge/generated.js").CHROME_DEFAULTS)["pullRequests"]} PullRequestList */

export const SIGNED_OUT = "GitHub is not signed in";

/** @type {Record<string, import("../core/icons.js").IconName>} */
const ICONS = {
  [PULL_REQUEST_STATES.OPEN]: "pull-open",
  [PULL_REQUEST_STATES.DRAFT]: "pull-draft",
  [PULL_REQUEST_STATES.MERGED]: "pull-merged",
  [PULL_REQUEST_STATES.CLOSED]: "pull-closed",
};

/** What the Pull requests tier draws: rows, the signed-out line, or nothing.
 * @param {PullRequestList | undefined} list
 * @returns {{rows: {key: string, title: string, number: string, url: string, state: string, icon: import("../core/icons.js").IconName, repo: string, fullRepo: string}[], note: string} | null} */
export function pullRows(list) {
  if (list?.signedOut) return { rows: [], note: SIGNED_OUT };
  const items = list?.items ?? [];
  if (!items.length) return null;
  const rows = items.map((pr) => ({
    key: `${pr.repo}#${pr.number}`,
    title: pr.title,
    number: `#${pr.number}`,
    url: pr.url,
    state: pr.state,
    icon: ICONS[pr.state],
    repo: pr.repo.slice(pr.repo.indexOf("/") + 1),
    fullRepo: pr.repo,
  }));
  return { rows, note: "" };
}
