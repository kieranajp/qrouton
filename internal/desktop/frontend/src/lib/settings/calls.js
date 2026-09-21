import { SETTINGS_LOAD, SETTINGS_QUIT, SETTINGS_SAVE, SETTINGS_VAULT_SESSION, SETTINGS_SAVE_VAULT_SESSION } from "../bridge/generated.js";
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
 * @property {VaultProfile[]} vaultProfiles
 * @property {Record<string,string>} vaultMappings
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

/** @typedef {{id: string, name: string, root: string}} VaultProfile */
/** @typedef {{readProfiles?: string[], repositories?: string[], destination?: string, publicationDisabled?: boolean}} VaultSelection */
/** @typedef {{session: string, workstream: string, selection: VaultSelection, repositories: string[], status: {scope?: {selectionRequired?: boolean, unmapped?: string[]}, profiles?: {profile: string, state: string, documents: number, invalid: number, unsupported: number, conflicts: number}[]}}} VaultSession */
/** @returns {Promise<VaultSession>} */
export const vaultSession = () => Call.ByName(SETTINGS_VAULT_SESSION);
/** @param {{session: string, workstream: string, selection: VaultSelection}} input
 * @returns {Promise<VaultSession>} */
export const saveVaultSession = (input) => Call.ByName(SETTINGS_SAVE_VAULT_SESSION, input);
