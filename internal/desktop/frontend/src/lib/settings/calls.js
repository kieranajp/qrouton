import {
  ASSEMBLY_RUNNERS,
  PLACEMENT_CHECK,
  SETTINGS_ADJUST_UI_SCALE,
  SETTINGS_GIT_HUB_STATUS,
  SETTINGS_INSTALL_LINEAR,
  SETTINGS_LOAD,
  SETTINGS_OPEN_CONFIG,
  SETTINGS_RELAUNCH,
  SETTINGS_RUNNERS,
  SETTINGS_SAVE,
} from "../bridge/generated.js";
import { Call } from "../wails.js";

/** @typedef {{path: string, snippet: string, replaces: boolean, error?: string}} LinearView */

/**
 * @typedef {object} SettingsInput
 * @property {{harness: string, model: string, reasoning: string} | null} reviewer
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
 * @property {{harness: string, model: string, reasoning: string} | null} [reviewer]
 * @property {string} [reviewerError]
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

/** @returns {Promise<{state: import("../bridge/generated.js").GitHubCredential, check: import("../bridge/generated.js").GitHubTokenCheck | "", login: string, variable: string}>} */
export const gitHubStatus = () => Call.ByName(SETTINGS_GIT_HUB_STATUS);

/**
 * @param {import("../bridge/generated.js").UIScaleAction} action
 * @returns {Promise<number>}
 */
export const adjustUIScale = (action) => Call.ByName(SETTINGS_ADJUST_UI_SCALE, action);

/** @typedef {{id: string, label: string, reviewer: {reasoningValues?: string[], reasoningEditable?: boolean, reasoningHelp?: string, modelHelp?: string}}} ReviewerRunner */

/** @returns {Promise<ReviewerRunner[]>} */
export const runners = () => Call.ByName(ASSEMBLY_RUNNERS);

/** @param {string} launch
 * @returns {Promise<ReviewerRunner[]>} */
export const candidateRunners = (launch) => Call.ByName(SETTINGS_RUNNERS, launch);

export const checkPlacement = () => Call.ByName(PLACEMENT_CHECK);
