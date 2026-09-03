// The chrome around the five first-run screens, kept pure: node --test is the
// whole frontend harness. What each screen is called and how it goes forward;
// the prose lives in the screens themselves.

import { addOrg } from "../settings/orgs.js";

const BACK = "← Back";

const NEEDS_OWNER = "Add at least one organisation or username to search.";

const SCREENS = [
  { caps: "", primary: "Show me →", back: false },
  { caps: "The one idea to know", primary: "Next →", back: true },
  { caps: "Where you will spend your time", primary: "Set it up →", back: true },
  { caps: "Question 1 of 2", primary: "Next →", back: true, owners: true },
  { caps: "Question 2 of 2", primary: "Find my repositories →", back: true },
];

export const title = "Welcome to qrouton";

export const total = SCREENS.length;

export const last = total - 1;

/** @param {number} [step] */
const screen = (step = 0) => SCREENS[step] ?? SCREENS[0];

/** caps is the label above the screen's heading; the first screen has none. */
export const caps = (step = 0) => screen(step).caps;

export const primary = (step = 0) => screen(step).primary;

/** back is the secondary label, or "" where there is nothing to go back to. */
export const back = (step = 0) => (screen(step).back ? BACK : "");

/** pip is which of the five pips is lit, which is the step itself. */
export const pip = (step = 0) => Math.min(Math.max(step, 0), last);

/** A valid uncommitted organization input counts because advancing commits it.
 * @param {number} [step]
 * @param {string[]} [orgs]
 * @param {string} [input] */
export const blocking = (step = 0, orgs = [], input = "") =>
  screen(step).owners && addOrg(orgs, input).length === 0 ? NEEDS_OWNER : "";
