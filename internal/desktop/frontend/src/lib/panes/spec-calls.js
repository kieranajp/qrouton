import { WINDOWS_CONTENT, WINDOWS_SAVE_SPEC, WINDOWS_SEND_SPEC_ANSWERS } from "../bridge/generated.js";
import { Call, call } from "../wails.js";

/** @param {string} id @param {string} hash The hash of the text the pane spliced into. @param {string} text */
export const saveSpec = (id, hash, text) => call(Call.ByName(WINDOWS_SAVE_SPEC, id, hash, text));

/** @param {string} id */
export const freshContent = (id) => call(Call.ByName(WINDOWS_CONTENT, id));

/** @param {string} id */
export const sendSpecAnswers = (id) => call(Call.ByName(WINDOWS_SEND_SPEC_ANSWERS, id));
