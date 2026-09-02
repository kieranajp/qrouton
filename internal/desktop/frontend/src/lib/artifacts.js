// One rule for every place a document names itself: a filled block of the kind's
// hue, short where space is short and long in a document head. A kind no
// taxonomy claims is the neutral sixth case, not an absence.

const KINDS = {
  PLAN: { tone: "var(--artifact-plan)", short: "PLAN", long: "PLAN" },
  SPEC: { tone: "var(--artifact-spec)", short: "SPEC", long: "SPEC" },
  RESEARCH: { tone: "var(--artifact-research)", short: "RSCH", long: "RESEARCH" },
  NOTE: { tone: "var(--artifact-note)", short: "NOTE", long: "NOTE" },
  EXPLAINER: { tone: "var(--artifact-explainer)", short: "EXPL", long: "EXPLAINER" },
};

const NEUTRAL = { tone: "var(--surface-raised)", short: "DOC", long: "DOCUMENT" };

/** @param {string | undefined} kind */
const entry = (kind) => KINDS[kind ?? ""] ?? NEUTRAL;

/** @param {string | undefined} kind */
export const artifactTone = (kind) => entry(kind).tone;

/** Crust on a hue, secondary on the neutral block: a tag is legible either way.
 * @param {string | undefined} kind */
export const artifactInk = (kind) =>
  entry(kind) === NEUTRAL ? "var(--text-secondary)" : "var(--text-on-accent)";

/**
 * @param {string | undefined} kind
 * @param {{id?: string, long?: boolean}} [options]
 */
export function artifactLabel(kind, { id = "", long = false } = {}) {
  if (long) return entry(kind).long;
  return id || entry(kind).short;
}
