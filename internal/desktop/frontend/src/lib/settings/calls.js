import { Call } from "../wails.js";

const SETTINGS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Settings";

/**
 * @typedef {object} SettingsFields
 * @property {string[]} orgs
 * @property {string} root
 * @property {string} editor
 * @property {string} launch
 * @property {string} linear
 * @property {string} [linearPath]
 * @property {string} [linearError]
 */

/** @returns {Promise<SettingsFields>} */
export const load = () => Call.ByName(SETTINGS_SERVICE + ".Load");

/**
 * @param {SettingsFields} input
 * @returns {Promise<{restartRequired: boolean}>}
 */
export const save = (input) => Call.ByName(SETTINGS_SERVICE + ".Save", input);

export const quit = () => Call.ByName(SETTINGS_SERVICE + ".Quit");
