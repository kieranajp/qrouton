import { DASHBOARD_PULL_STATES, PULL_REQUEST_REVIEWS, PULL_REQUEST_STATES, REPO_ROLES } from "../bridge/generated.js";
import { SIGNED_OUT } from "../overview/pulls.js";
import { stamp } from "../relative.js";
import { repositoryLine } from "../session/rail.js";

export const DASHBOARD_TITLE = "Dashboard";

export const MAX_COLUMNS = 4;
export const COLUMN_GAP = 14;
const COLUMN_MIN = 320;

/** How many columns of cards fit a grid this wide, at the UI scale as a fraction.
 * @param {number} width @param {number} [scale] */
export function columnCount(width, scale = 1) {
  const fit = Math.floor((width + COLUMN_GAP * scale) / ((COLUMN_MIN + COLUMN_GAP) * scale));
  return Math.max(1, Math.min(MAX_COLUMNS, Number.isFinite(fit) ? fit : 1));
}

/** @param {number} [pulls] the pull requests the card shows */
export const estimatedHeight = (pulls = 0) => 100 + 62 * pulls;

/** Expanding a card leaves its estimate alone, so no other card changes column under the pointer.
 * @param {{shown: unknown[], more: number}} section */
export const cardHeight = (section) => estimatedHeight(Math.min(section.shown.length + section.more, SHOWN_PULLS));

/** Each card's index, in the column that was shortest when it was placed.
 * A tie goes left, so the first row reads left to right in rail order.
 * @param {number[]} heights @param {number} columns @returns {number[][]} */
export function masonry(heights, columns) {
  const count = Math.max(1, columns);
  const placed = Array.from({ length: count }, () => /** @type {number[]} */ ([]));
  const filled = Array(count).fill(0);
  heights.forEach((height, index) => {
    const shortest = filled.indexOf(Math.min(...filled));
    placed[shortest].push(index);
    filled[shortest] += height;
  });
  return placed;
}

/** @param {{name?: string}[]} [repos] */
export const repoList = (repos = []) =>
  repos.length ? repos.map((repo) => repo.name).join(", ") : repositoryLine(repos).name;

export const REFRESH = "Refresh";
export const GITHUB_SETTINGS = "GitHub settings";
export const CHECKING = "Checking GitHub…";
export const NO_PULLS = "No pull requests";
export const NO_SOURCE = "No branch to check";
export const UNAVAILABLE = "Couldn't load pull requests";
export const PARTIAL = "Couldn't load every repository";
/** @type {Readonly<Record<string, string>>} */
const PULL_STATE_NOTES = Object.freeze({
  [DASHBOARD_PULL_STATES.ANSWERED]: NO_PULLS,
  [DASHBOARD_PULL_STATES.UNAVAILABLE]: UNAVAILABLE,
  [DASHBOARD_PULL_STATES.NONE]: NO_SOURCE,
});
export const SHOWN_PULLS = 4;

/** @param {number} count */
export const showMore = (count) => `Show ${count} more`;

/** @param {number} count @param {string} session */
export const showMoreLabel = (count, session) => `${showMore(count)} pull requests in ${session}`;

export const STATE_WORDS = Object.freeze({
  [PULL_REQUEST_STATES.OPEN]: "Open",
  [PULL_REQUEST_STATES.DRAFT]: "Draft",
  [PULL_REQUEST_STATES.MERGED]: "Merged",
  [PULL_REQUEST_STATES.CLOSED]: "Closed",
});

const REVIEW_TAGS = Object.freeze({
  [PULL_REQUEST_REVIEWS.APPROVED]: { label: "Approved", tone: "success" },
  [PULL_REQUEST_REVIEWS.CHANGES_REQUESTED]: { label: "Changes requested", tone: "waiting" },
});
const CHECKS_FAILING = Object.freeze({ label: "Checks failing", tone: "failed" });

const MINUTE = 60000;

/** @typedef {{repo: string, number: number, title: string, url: string, state: string, author?: string, review?: string, checksFailing?: boolean, updatedAt?: string}} PullRequest */
/** @typedef {{name: string, role?: string, measured?: boolean, commits?: number, insertions?: number, deletions?: number}} RepoStat */
/** @typedef {{viewer?: string, checkedAt?: string, refreshing?: boolean, failed?: boolean, signedOut?: boolean, pausedUntil?: string, sessions?: Record<string, {pullRequests?: PullRequest[] | null, pullState?: string, repos?: RepoStat[] | null}>}} DashboardStatus */

/** Go marshals never as year one, which parses to a negative time.
 * @param {string | undefined} at */
const time = (at) => {
  const ms = at ? Date.parse(at) : NaN;
  return ms > 0 ? ms : 0;
};

const open = (/** @type {PullRequest} */ pr) =>
  pr.state === PULL_REQUEST_STATES.OPEN || pr.state === PULL_REQUEST_STATES.DRAFT;

/** @param {PullRequest[] | null | undefined} pulls @param {string | undefined} viewer */
export function ownPulls(pulls, viewer) {
  if (!viewer) return [];
  return (pulls ?? [])
    .filter((pr) => pr.author === viewer)
    .sort((a, b) => Number(open(b)) - Number(open(a)) || time(b.updatedAt) - time(a.updatedAt));
}

/** @param {PullRequest} pr @returns {{label: string, tone: string}[]} */
export function pullTags(pr) {
  const review = pr.review ? REVIEW_TAGS[pr.review] : undefined;
  return [review, open(pr) && pr.checksFailing ? CHECKS_FAILING : undefined].filter((tag) => tag !== undefined);
}

/** A card reads its own branches' answer, never the whole refresh's.
 * @param {DashboardStatus} status @param {string} slug @param {number} now @param {{expanded?: boolean}} [card]
 * @returns {{shown: PullRequest[], more: number, note: string}} */
export function pullSection(status, slug, now, { expanded = false } = {}) {
  const session = status.sessions?.[slug];
  const rows = ownPulls(session?.pullRequests, status.viewer);
  if (rows.length) {
    const shown = expanded ? rows : rows.slice(0, SHOWN_PULLS);
    const note = session?.pullState === DASHBOARD_PULL_STATES.UNAVAILABLE ? PARTIAL : "";
    return { shown, more: rows.length - shown.length, note };
  }
  const note = session?.pullState ? PULL_STATE_NOTES[session.pullState] : undefined;
  if (note) return { shown: [], more: 0, note };
  // The header already says why GitHub has not answered, and it will not answer by waiting.
  if (status.signedOut || status.failed || paused(status, now)) return { shown: [], more: 0, note: "" };
  return { shown: [], more: 0, note: CHECKING };
}

/** @param {RepoStat[] | null | undefined} repos */
export function gitLine(repos) {
  const measured = (repos ?? []).filter((repo) => repo.role === REPO_ROLES.EDITING && repo.measured);
  if (!measured.length) return "";
  const sum = (/** @type {"commits" | "insertions" | "deletions"} */ key) =>
    measured.reduce((total, repo) => total + (repo[key] ?? 0), 0);
  const commits = sum("commits");
  if (!commits) return "no commits";
  return `${commits} commit${commits === 1 ? "" : "s"} · +${sum("insertions")} −${sum("deletions")}`;
}

/** @param {DashboardStatus} status @param {number} now */
export const paused = (status, now) => time(status.pausedUntil) > now;

/** @param {DashboardStatus} status @param {number} now */
export const refreshDisabled = (status, now) => Boolean(status.refreshing) || paused(status, now);

/** @param {DashboardStatus} status @param {number} now
 * @returns {{text: string, warn: boolean, settings: boolean}} */
export function freshness(status, now) {
  const checked = time(status.checkedAt);
  if (status.signedOut) return { text: SIGNED_OUT, warn: false, settings: true };
  if (paused(status, now)) return { text: `GitHub rate limit · next try ${stamp(time(status.pausedUntil), now)}`, warn: true, settings: false };
  if (status.failed) {
    return { text: checked ? `Couldn't reach GitHub · showing ${stamp(checked, now)}` : "Couldn't reach GitHub", warn: true, settings: false };
  }
  if (!checked) return { text: "", warn: false, settings: false };
  return { text: now - checked < MINUTE ? "Updated just now" : `As of ${stamp(checked, now)}`, warn: false, settings: false };
}
