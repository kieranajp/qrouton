import { BUG_REPORTS_LOAD, BUG_REPORTS_CONFIRM, BUG_REPORTS_CANCEL } from "../bridge/generated.js";
import { Call } from "../wails.js";

export const loadReport = (slug, id) => Call.ByName(BUG_REPORTS_LOAD, slug, id);
export const confirmReport = (slug, id) => Call.ByName(BUG_REPORTS_CONFIRM, slug, id);
export const cancelReport = (slug, id) => Call.ByName(BUG_REPORTS_CANCEL, slug, id);
