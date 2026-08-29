// The updater's lifecycle, as the framework emits it. The gate draws one line
// from these rather than a log: what it is doing, and how far in.

export const CHECKING = "wails:updater:check-started";
export const DOWNLOADING = "wails:updater:download-started";
export const PROGRESS = "wails:updater:download-progress";
export const VERIFYING = "wails:updater:verifying";
export const INSTALLING = "wails:updater:installing";
export const READY = "wails:updater:update-ready";
export const FAILED = "wails:updater:error";

export const STAGES = [CHECKING, DOWNLOADING, PROGRESS, VERIFYING, INSTALLING, READY, FAILED];

const LABELS = {
  [CHECKING]: "Looking for the current release",
  [DOWNLOADING]: "Downloading",
  [PROGRESS]: "Downloading",
  [VERIFYING]: "Checking the download",
  [INSTALLING]: "Installing",
  [READY]: "Restarting into the new version",
  [FAILED]: "Could not update — retrying shortly",
};

/** @typedef {{label: string, percent: number, failed: boolean}} Stage */

/** staged is the gate before the updater has said anything. */
export const staged = () => ({ label: LABELS[CHECKING], percent: 0, failed: false });

/**
 * progressed folds one lifecycle event into the line on screen. A download
 * reports bytes rather than a percentage, and a total of zero is a server that
 * did not send a length — which is a bar that cannot move rather than one at
 * zero, so the previous width is kept.
 *
 * @param {Stage} stage
 * @param {string} name
 * @param {{written?: number, total?: number}} [payload]
 * @returns {Stage}
 */
export function progressed(stage, name, payload) {
  const label = LABELS[name];
  if (!label) return stage;
  if (name !== PROGRESS) {
    return { label, percent: name === READY ? 100 : stage.percent, failed: name === FAILED };
  }
  return { label, percent: percentOf(payload, stage.percent), failed: false };
}

function percentOf(payload, previous) {
  const total = Number(payload?.total ?? 0);
  const written = Number(payload?.written ?? 0);
  if (!(total > 0) || !(written >= 0)) return previous;
  return Math.max(0, Math.min(100, Math.round((written / total) * 100)));
}
