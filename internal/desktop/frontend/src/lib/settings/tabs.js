import { SETTINGS_FIELDS } from "../bridge/generated.js";

/** @type {ReadonlyArray<{key: string, label: string, restart?: boolean}>} */
export const TABS = Object.freeze([
  Object.freeze({ key: "general", label: "General" }),
  Object.freeze({ key: "github", label: "GitHub" }),
  Object.freeze({ key: "folders", label: "Folders", restart: true }),
  Object.freeze({ key: "search", label: "Search" }),
  Object.freeze({ key: "stickers", label: "Stickers" }),
  Object.freeze({ key: "advanced", label: "Advanced" }),
]);

const FIELD_TABS = Object.freeze({
  [SETTINGS_FIELDS.EDITOR]: "general",
  [SETTINGS_FIELDS.UI_SCALE]: "general",
  [SETTINGS_FIELDS.ORGS]: "github",
  [SETTINGS_FIELDS.ROOT]: "folders",
  [SETTINGS_FIELDS.THOUGHTS]: "folders",
  [SETTINGS_FIELDS.STICKERS]: "stickers",
  [SETTINGS_FIELDS.LAUNCH]: "advanced",
  [SETTINGS_FIELDS.REVIEWER]: "advanced",
});

/** A sticker row's field is named stickers.<index>, so only the part before the dot picks the tab.
 * @param {string} field
 * @returns {string | undefined} */
export const tabOf = (field) => FIELD_TABS[field.split(".")[0]];

/** @param {Record<string, string>} fields */
export const erroredTabs = (fields) =>
  new Set(Object.keys(fields).filter((field) => fields[field]).map(tabOf).filter(Boolean));

/** The first errored tab in index order, or undefined when nothing errored.
 * @param {Record<string, string>} fields */
export function firstErroredTab(fields) {
  const errored = erroredTabs(fields);
  return TABS.find((tab) => errored.has(tab.key))?.key;
}

/** Arrow keys wrap through the index; Home and End jump to its ends.
 * @param {string} current
 * @param {string} key
 * @returns {string | undefined} */
export function stepTab(current, key) {
  const at = TABS.findIndex((tab) => tab.key === current);
  const last = TABS.length - 1;
  const to = { ArrowDown: at + 1, ArrowUp: at - 1, Home: 0, End: last }[key];
  if (to === undefined) return undefined;
  return TABS[(to + TABS.length) % TABS.length].key;
}
