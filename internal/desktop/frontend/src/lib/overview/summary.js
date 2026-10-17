import { relative } from "../relative.js";
import { STAGE_NAMES } from "./tree.js";

/** @typedef {(typeof import("../bridge/generated.js").CHROME_DEFAULTS)["sessionSummary"]} SessionSummary */

const EMPTY = { text: "No summary yet", meta: "Written when the first lead starts", empty: true };

/** @param {SessionSummary | undefined} summary @param {number} [now] */
export function summaryMeta(summary, now = Date.now()) {
  const text = summary?.text?.trim() ?? "";
  if (!text) return EMPTY;
  const ago = relative(summary?.at ?? "", "ago", now);
  const stage = STAGE_NAMES[summary?.stage ?? ""];
  const meta = [stage && `As of ${stage}`, ago].filter(Boolean).join(" · ");
  return { text, meta, empty: false };
}
