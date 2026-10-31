import {
  PRESENTER_CLOSE,
  PRESENTER_OPEN,
  PRESENTER_SHOW,
  WINDOWS_FOCUS_ASSET,
  WINDOWS_RENDER_DIAGRAMS,
  WINDOWS_REPORT_VIEWPORT,
} from "../bridge/generated.js";
import { Call, call } from "../wails.js";

/** @param {string} id @param {number} request */
export const renderDiagrams = (id, request) => call(Call.ByName(WINDOWS_RENDER_DIAGRAMS, id, request));

/** @param {string} id @param {object} report */
export const reportViewport = (id, report) => call(Call.ByName(WINDOWS_REPORT_VIEWPORT, id, report));

/** @param {string} slug @param {string} id @param {number} index */
export const focusAsset = (slug, id, index) => call(Call.ByName(WINDOWS_FOCUS_ASSET, slug, id, index));

/** @param {{index: number, total: number, title: string, html: string}} note */
export const showNote = (note) => call(Call.ByName(PRESENTER_SHOW, note));

export const openNotes = () => call(Call.ByName(PRESENTER_OPEN));

export const closeNotes = () => call(Call.ByName(PRESENTER_CLOSE));
