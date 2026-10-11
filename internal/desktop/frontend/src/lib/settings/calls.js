import {
  SETTINGS_ADJUST_UI_SCALE,
  SETTINGS_GIT_HUB_STATUS,
  SETTINGS_INSTALL_LINEAR,
  SETTINGS_LOAD,
  SETTINGS_OPEN_CONFIG,
  SETTINGS_RELAUNCH,
  SETTINGS_SAVE,
} from "../bridge/generated.js";
import { Call } from "../wails.js";

/** @typedef {{path: string, snippet: string, replaces: boolean, error?: string}} LinearView */

/**
 * @typedef {object} SettingsInput
 * @property {string[]} orgs
 * @property {string} root
 * @property {string} editor
 * @property {string} launch
 * @property {import("../session/stickers.js").Sticker[]} stickers
 * @property {boolean} chime
 * @property {number} uiScale
 * @property {{default: string, roots: {id: string, path: string, orgs: string}[]}} thoughts
 */

/**
 * @typedef {object} SettingsView
 * @property {string[]} orgs
 * @property {string} root
 * @property {string} editor
 * @property {string} launch
 * @property {import("../session/stickers.js").Sticker[]} stickers
 * @property {boolean} chime
 * @property {number} uiScale
 * @property {number[]} [uiScaleSteps]
 * @property {{default: string, derived?: string, roots: {id: string, path: string, orgs: string, missing?: boolean}[]}} thoughts
 * @property {LinearView} [linear]
 * @property {{root: boolean, orgs: boolean}} [pinned]
 * @property {{id: string, label: string}[]} [runners]
 */

/** @returns {Promise<SettingsView>} */
export const load = () => Call.ByName(SETTINGS_LOAD);

/**
 * @param {SettingsInput} input
 * @returns {Promise<{restartRequired: boolean}>}
 */
export const save = (input) => Call.ByName(SETTINGS_SAVE, input);

export const relaunch = () => Call.ByName(SETTINGS_RELAUNCH);

export const openConfig = () => Call.ByName(SETTINGS_OPEN_CONFIG);

/** @returns {Promise<LinearView>} */
export const installLinear = () => Call.ByName(SETTINGS_INSTALL_LINEAR);

/** @returns {Promise<{state: import("../bridge/generated.js").GitHubCredential, login: string, variable: string}>} */
export const gitHubStatus = () => Call.ByName(SETTINGS_GIT_HUB_STATUS);

/**
 * @param {import("../bridge/generated.js").UIScaleAction} action
 * @returns {Promise<number>}
 */
export const adjustUIScale = (action) => Call.ByName(SETTINGS_ADJUST_UI_SCALE, action);
