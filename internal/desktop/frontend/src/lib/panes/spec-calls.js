import { WINDOWS_CONTENT, WINDOWS_SAVE_ANSWERS, WINDOWS_SEND_AUDIT_TRIAGE, WINDOWS_SEND_SPEC_ANSWERS } from "../bridge/generated.js";
import { Call, call } from "../wails.js";

/** @param {string} id @param {string} hash The hash of the text the pane spliced into. @param {string} text */
export const saveAnswers = (id, hash, text) => call(Call.ByName(WINDOWS_SAVE_ANSWERS, id, hash, text));

/** @param {string} id */
export const freshContent = (id) => call(Call.ByName(WINDOWS_CONTENT, id));

/** @param {string} id */
export const sendSpecAnswers = (id) => call(Call.ByName(WINDOWS_SEND_SPEC_ANSWERS, id));

/** @param {string} id */
export const sendAuditTriage = (id) => call(Call.ByName(WINDOWS_SEND_AUDIT_TRIAGE, id));
