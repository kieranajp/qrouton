// The chrome around the first-run screens. What each screen is called
// and how it goes forward; the prose lives in the screens themselves.

import { addOrg } from "../settings/orgs.js";

const BACK = "Back";

const NEEDS_OWNER = "Add at least one organisation or username to search.";

/** @typedef {(typeof SCREENS)[number]["key"]} Screen */
const SCREENS = /** @type {const} */ ([
  { key: "welcome", caps: "", primary: "Show me", back: false },
  { key: "session", caps: "The one idea to know", primary: "Next", back: true },
  { key: "panels", caps: "Where you will spend your time", primary: "Set it up", back: true },
  { key: "orgs", caps: "Question 1 of 3", primary: "Next", back: true, owners: true, field: "orgs" },
  { key: "root", caps: "Question 2 of 3", primary: "Next", back: true, field: "root" },
  { key: "thoughts", caps: "Question 3 of 3", primary: "Next", back: true, field: "thoughts" },
  { key: "ollama", caps: "Optional", primary: "Find my repositories", back: true },
]);

export const title = "Welcome to qrouton";

export const total = SCREENS.length;

export const last = total - 1;

/** @param {number} [step] */
const screen = (step = 0) => SCREENS[step] ?? SCREENS[0];

/** caps is the label above the screen's heading; the first screen has none. */
export const screenKey = (step = 0) => screen(step).key;

export const caps = (step = 0) => screen(step).caps;

export const primary = (step = 0) => screen(step).primary;

/** @type {{primary: import("../core/icons.js").IconName, back: import("../core/icons.js").IconName}} */
export const icons = { primary: "forward", back: "back" };

/** back is the secondary label, or "" where there is nothing to go back to. */
export const back = (step = 0) => (screen(step).back ? BACK : "");

/** pip is which pip is lit, which is the step itself. */
export const pip = (step = 0) => Math.min(Math.max(step, 0), last);

/** A valid uncommitted organization input counts because advancing commits it.
 * @param {number} [step]
 * @param {string[]} [orgs]
 * @param {string} [input] */
export const blocking = (step = 0, orgs = [], input = "") => {
  const selected = screen(step);
  return "owners" in selected && selected.owners && addOrg(orgs, input).length === 0 ? NEEDS_OWNER : "";
};

/** stepFor is the screen that asks a refused field, or -1 when none does.
 * @param {string} [field] */
export const stepFor = (field = "") => SCREENS.findIndex((s) => "field" in s && s.field === field);
