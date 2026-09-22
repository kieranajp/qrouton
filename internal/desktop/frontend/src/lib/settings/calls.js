import { SETTINGS_SELECT_VAULT_IMPORT_CORPUS, SETTINGS_PREVIEW_VAULT_CORPUS, SETTINGS_VAULT_CORPUS_ENTRY, SETTINGS_LOAD, SETTINGS_QUIT, SETTINGS_SAVE, SETTINGS_VAULT_SESSION, SETTINGS_SAVE_VAULT_SESSION, SETTINGS_VAULT_SETUP, SETTINGS_DOWNLOAD_VAULT_MODEL, SETTINGS_CANCEL_VAULT_DOWNLOAD, SETTINGS_RETRY_VAULT_INDEX, SETTINGS_SELECT_VAULT_IMPORT_SOURCES, SETTINGS_CLEAR_VAULT_IMPORT_SOURCES, SETTINGS_PREVIEW_VAULT_IMPORT, SETTINGS_CONFIRM_VAULT_IMPORT, SETTINGS_VAULT_IMPORT_STATUS, SETTINGS_RETRY_VAULT_IMPORTS, SETTINGS_REVALIDATE_VAULT_IMPORT_DESTINATION } from "../bridge/generated.js";
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

/** @typedef {{state: string, dependency?: string, error?: string, indexed: number, pending: number, excluded: number, unresolved: number, competing: number, chunks: number, completed: number, total: number}} VaultIndex */
/** @typedef {{model: string, installed: boolean, status: {enabled: boolean, profiles: {profile: string, documents: number, invalid: number, unsupported: number, conflicts: number, index?: VaultIndex}[]}, download: {state: string, error?: string, progress: {status: string, total: number, completed: number}}}} VaultSetup */
/** @returns {Promise<VaultSetup>} */
export const vaultSetup = () => Call.ByName(SETTINGS_VAULT_SETUP);
export const downloadVaultModel = () => Call.ByName(SETTINGS_DOWNLOAD_VAULT_MODEL);
export const cancelVaultDownload = () => Call.ByName(SETTINGS_CANCEL_VAULT_DOWNLOAD);
/** @param {string} profile */
export const retryVaultIndex = (profile) => Call.ByName(SETTINGS_RETRY_VAULT_INDEX, profile);

export const selectVaultImportSources = (kind) => Call.ByName(SETTINGS_SELECT_VAULT_IMPORT_SOURCES, kind);
export const previewVaultImport = (input) => Call.ByName(SETTINGS_PREVIEW_VAULT_IMPORT, input);
export const confirmVaultImport = (preview, selected) => Call.ByName(SETTINGS_CONFIRM_VAULT_IMPORT, preview, selected);
export const vaultImportStatus = () => Call.ByName(SETTINGS_VAULT_IMPORT_STATUS);
export const retryVaultImports = (profile) => Call.ByName(SETTINGS_RETRY_VAULT_IMPORTS, profile);
export const revalidateVaultImportDestination = (id) => Call.ByName(SETTINGS_REVALIDATE_VAULT_IMPORT_DESTINATION, id);

export const clearVaultImportSources = () => Call.ByName(SETTINGS_CLEAR_VAULT_IMPORT_SOURCES);

export const selectVaultImportCorpus = () => Call.ByName(SETTINGS_SELECT_VAULT_IMPORT_CORPUS);
export const previewVaultCorpus = (input) => Call.ByName(SETTINGS_PREVIEW_VAULT_CORPUS, input);
export const vaultCorpusEntry = (preview, key) => Call.ByName(SETTINGS_VAULT_CORPUS_ENTRY, preview, key);
