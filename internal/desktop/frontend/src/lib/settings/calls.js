import {
  PLACEMENT_CHECK,
  PROFILES_ADD,
  PROFILES_DELETE,
  PROFILES_RENAME,
  PROFILES_SIGN_IN,
  PROFILES_STATUS,
  PROFILES_SUGGEST,
  PROFILES_VIEW,
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

/** @returns {Promise<{state: import("../bridge/generated.js").GitHubCredential, check: import("../bridge/generated.js").GitHubTokenCheck | "", login: string, variable: string}>} */
export const gitHubStatus = () => Call.ByName(SETTINGS_GIT_HUB_STATUS);

/**
 * @param {import("../bridge/generated.js").UIScaleAction} action
 * @returns {Promise<number>}
 */
export const adjustUIScale = (action) => Call.ByName(SETTINGS_ADJUST_UI_SCALE, action);

export const checkPlacement = () => Call.ByName(PLACEMENT_CHECK);

/** @typedef {{id: string, label: string, dir: string, sessions: number, uncounted?: boolean}} ProfileRow */
/** @typedef {{id: string, label: string, credentials: string[], warning?: string, profiles: ProfileRow[]}} RunnerProfiles */
/** @typedef {{runners: RunnerProfiles[], sessionShown: boolean}} ProfilesView */
/** @typedef {{runner: string, profile: string, state: import("../bridge/generated.js").SignInState, email?: string}} ProfileStatus */

/** @returns {Promise<ProfilesView>} */
export const profilesView = () => Call.ByName(PROFILES_VIEW);

/** @param {string} runner @param {string} label @returns {Promise<{id: string, dir: string}>} */
export const suggestProfile = (runner, label) => Call.ByName(PROFILES_SUGGEST, runner, label);

/** @param {{runner: string, label: string, dir: string}} input @returns {Promise<ProfilesView>} */
export const addProfile = (input) => Call.ByName(PROFILES_ADD, input);

/** @param {string} runner @param {string} id @param {string} label @returns {Promise<ProfilesView>} */
export const renameProfile = (runner, id, label) => Call.ByName(PROFILES_RENAME, runner, id, label);

/** @param {string} runner @param {string} id @returns {Promise<ProfilesView>} */
export const deleteProfile = (runner, id) => Call.ByName(PROFILES_DELETE, runner, id);

/** @returns {Promise<ProfileStatus[]>} */
export const profileStatus = () => Call.ByName(PROFILES_STATUS);

/** The id of the tab the sign-in runs in.
 * @param {string} runner @param {string} id @returns {Promise<string>} */
export const signIn = (runner, id) => Call.ByName(PROFILES_SIGN_IN, runner, id);
