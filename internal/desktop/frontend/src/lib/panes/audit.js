import { AUDIT_HEADINGS } from "../bridge/generated.js";
import { plainText, sliceSections } from "./sections.js";
import { lastFilled, spliceLabelled } from "./spec.js";

export const TELLS = [AUDIT_HEADINGS.DUPLICATED, AUDIT_HEADINGS.DEAD, AUDIT_HEADINGS.HOMEGROWN];
export const CHOICES = ["accept", "reject", "defer"];
export const TRIAGED = { accept: "Accepted", reject: "Rejected", defer: "Deferred" };
export const CONFIDENCES = ["high", "medium", "low"];

const FINDING = /^(F\d+)\s*[—–:-]\s*(\S.*?)\s*$/;
const FIELD = /^(Tell|Locations|Evidence|Verdict|Proposed fix|Behaviour risk|Confidence|Source):[ \t]?(.*)$/;
const TRIAGE = /^Triage:[ \t]?(.*)$/;
const STEP = /\bstep\s*([1-5])\b/i;
const KEYS = {
  Tell: "tell",
  Locations: "locations",
  Evidence: "evidence",
  Verdict: "verdict",
  "Proposed fix": "fix",
  "Behaviour risk": "risk",
  Confidence: "confidence",
  Source: "source",
};

/** @param {string} line */
const isChoice = (line) => CHOICES.includes(line.trim());

/** @typedef {{from: number, to: number, raw: string, choice: string, note: string}} Triage */
/** @typedef {{tell: string, locations: string[], evidence: string, verdict: string, fix: string, risk: string, confidence: string, source: string}} Fields */
/** @typedef {{id: string, title: string, section: string, from: number, to: number, fields: Fields, step: number, triage: Triage | null}} Finding */

/** A choice counts only when it is all the Triage line holds; anything else is a note with no choice.
 * @param {string} first @param {string} rest */
export function readTriage(first, rest) {
  if (isChoice(first)) return { choice: first.trim(), note: rest.trim() };
  return { choice: "", note: [first, rest].join("\n").trim() };
}

/** @param {any} node */
const line = (node) => node.position?.start?.line ?? 0;

/** @param {string[]} lines @param {number} from @param {number} to @param {string} section @param {RegExpExecArray} named */
function readFinding(lines, from, to, section, named) {
  /** @type {Record<string, string>} */
  const raw = {};
  let field = "";
  let triageAt = 0;
  for (let at = from + 1; at <= to; at++) {
    const source = lines[at - 1];
    if (TRIAGE.test(source)) {
      triageAt = at;
      break;
    }
    const opened = FIELD.exec(source);
    if (opened) {
      field = KEYS[opened[1]];
      raw[field] = opened[2];
    } else if (field) {
      raw[field] += `\n${source}`;
    }
  }

  let triage = null;
  if (triageAt) {
    const end = lastFilled(lines, triageAt, to);
    const first = TRIAGE.exec(lines[triageAt - 1])?.[1] ?? "";
    const rest = lines.slice(triageAt, end).join("\n");
    triage = { from: triageAt, to: end, raw: [first, rest].join("\n").trim(), ...readTriage(first, rest) };
  }

  const text = (key) => (raw[key] ?? "").trim();
  const fields = {
    tell: text("tell"),
    locations: text("locations").split(",").map((at) => at.trim()).filter(Boolean),
    evidence: text("evidence"),
    verdict: text("verdict"),
    fix: text("fix"),
    risk: text("risk"),
    confidence: text("confidence").toLowerCase(),
    source: text("source"),
  };
  const step = Number(STEP.exec(fields.verdict)?.[1] ?? 0);
  return { id: named[1], title: named[2], section, from, to, fields, step, triage };
}

/** An audit is triageable once it holds one of the tell sections.
 * @param {string} text */
export function parseAudit(text) {
  const { title, preamble, sections } = sliceSections(text);
  const lines = text.split("\n");
  const tellOf = (name) => TELLS.find((tell) => tell.toLowerCase() === name.toLowerCase());

  /** @type {Finding[]} */
  const findings = [];
  const tells = [];
  for (const section of sections) {
    const tell = tellOf(section.name);
    if (!tell) continue;
    const heads = section.nodes.filter((node) => node.type === "heading" && node.depth === 3);
    let count = 0;
    heads.forEach((heading, at) => {
      const named = FINDING.exec(plainText(heading).trim());
      if (!named) return;
      const to = heads[at + 1] ? line(heads[at + 1]) - 1 : section.to;
      findings.push(readFinding(lines, line(heading), to, tell, named));
      count++;
    });
    tells.push({ name: tell, from: section.from, to: section.to, count });
  }

  return {
    title,
    preamble,
    isAudit: tells.length > 0,
    findings,
    tells,
    sections: sections
      .filter((section) => !tellOf(section.name))
      .map(({ name, from, to }) => ({ name, from, to })),
  };
}

/** High, medium, low, then anything else, each in document order.
 * @template {{fields: {confidence: string}}} F @param {F[]} findings */
export function byConfidence(findings) {
  const rank = (finding) => {
    const at = CONFIDENCES.indexOf(finding.fields.confidence);
    return at < 0 ? CONFIDENCES.length : at;
  };
  return findings.map((finding, at) => ({ finding, at })).sort((a, b) => rank(a.finding) - rank(b.finding) || a.at - b.at).map(({ finding }) => finding);
}

/** @param {string} text @param {Finding} finding @param {{choice: string, note: string}} triage */
export const spliceTriage = (text, finding, triage) =>
  spliceLabelled(text, { label: "Triage", isChoice, block: finding, written: finding.triage }, triage);

/** Lays unsaved triage back over a freshly parsed audit. An id is never reused,
 * so a retitled finding keeps its draft; a draft whose finding is gone is detached.
 * @template {{id: string}} D
 * @param {{findings: Finding[]}} parsed @param {Map<string, D>} drafts */
export function reapply(parsed, drafts) {
  const ids = new Set(parsed.findings.map((finding) => finding.id));
  return {
    findings: parsed.findings.map((finding) => ({ finding, draft: drafts.get(finding.id) ?? null })),
    detached: [...drafts.entries()].filter(([id]) => !ids.has(id)).map(([, draft]) => draft),
  };
}
