import { PLACEMENT_CLOSE, PLACEMENT_MOVE, PLACEMENT_REPORT } from "../bridge/generated.js";
import { Call } from "../wails.js";

/**
 * @typedef {{id: string, path: string}} PlacementRoot
 * @typedef {{slug: string, name: string, state: string, reason?: string, detail?: string, from: PlacementRoot, to: PlacementRoot, leaving?: boolean}} PlacementItem
 * @typedef {{misplaced: PlacementItem[], skipped: PlacementItem[], leavingNote: string, relaunchNote: string}} PlacementView
 * @typedef {{slug: string, result: string, detail?: string}} PlacementOutcome
 */

/** @returns {Promise<PlacementView>} */
export const report = () => Call.ByName(PLACEMENT_REPORT);

/**
 * @param {string[]} slugs
 * @returns {Promise<PlacementOutcome[]>}
 */
export const move = (slugs) => Call.ByName(PLACEMENT_MOVE, slugs);

export const close = () => Call.ByName(PLACEMENT_CLOSE);
