import { SETTINGS_LOAD, SETTINGS_QUIT, SETTINGS_SAVE } from "../bridge/generated.js";
import { Call } from "../wails.js";

/**
 * @typedef {object} StickerLabels
 * @property {string} star
 * @property {string} bookmark
 * @property {string} question
 * @property {string} exclamation
 */

/**
 * @typedef {object} SettingsFields
 * @property {string[]} orgs
 * @property {string} root
 * @property {string} editor
 * @property {string} launch
 * @property {string} linear
 * @property {StickerLabels} stickerLabels
 * @property {string} [linearPath]
 * @property {string} [linearError]
 */

/** @returns {Promise<SettingsFields>} */
export const load = () => Call.ByName(SETTINGS_LOAD);

/**
 * @param {SettingsFields} input
 * @returns {Promise<{restartRequired: boolean}>}
 */
export const save = (input) => Call.ByName(SETTINGS_SAVE, input);

export const quit = () => Call.ByName(SETTINGS_QUIT);
