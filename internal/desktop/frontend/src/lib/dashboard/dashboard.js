import { MODE_LABELS } from "../bridge/generated.js";
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

/** @param {{mode?: string}} row */
export const guided = (row) => row.mode === MODE_LABELS.RPI;
