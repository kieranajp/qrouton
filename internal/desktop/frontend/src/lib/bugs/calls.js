import { BUG_REPORTS_LOAD, BUG_REPORTS_CONFIRM, BUG_REPORTS_CANCEL } from "../bridge/generated.js";
import { Call } from "../wails.js";

/** @typedef {{id: string, status: string, message: string, url?: string, number?: number}} BugReport */
/** @typedef {{report: BugReport, title: string, body: string, destination: string, reviewLabel: string, createLabel: string, cancelLabel: string, closeLabel: string, unknownMessage: string}} BugReportPreview */

/** @returns {Promise<BugReportPreview>} */
export const loadReport = (slug, id) => Call.ByName(BUG_REPORTS_LOAD, slug, id);
/** @returns {Promise<BugReport>} */
export const confirmReport = (slug, id) => Call.ByName(BUG_REPORTS_CONFIRM, slug, id);
export const cancelReport = (slug, id) => Call.ByName(BUG_REPORTS_CANCEL, slug, id);
