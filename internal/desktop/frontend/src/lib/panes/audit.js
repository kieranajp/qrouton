import { AUDIT_HEADINGS } from "../bridge/generated.js";
import { plainText, sliceSections } from "./sections.js";
import { lastFilled, spliceLabelled } from "./spec.js";

export const TELLS = [AUDIT_HEADINGS.DUPLICATED, AUDIT_HEADINGS.DEAD, AUDIT_HEADINGS.HOMEGROWN, AUDIT_HEADINGS.VOLKSWAGEN];
export const CHOICES = ["accept", "reject", "defer"];
export const TRIAGED = { accept: "Accepted", reject: "Rejected", defer: "Deferred" };
export const CONFIDENCES = ["high", "medium", "low"];
export const IMPACTS = ["high", "medium", "low"];

const FINDING = /^(F\d+)\s*[—–:-]\s*(\S.*?)\s*$/;
const FIELD = /^(Tell|Locations|Evidence|Verdict|Proposed fix|Behaviour risk|Impact|Confidence|Source):[ \t]?(.*)$/;
const TRIAGE = /^Triage:[ \t]?(.*)$/;
const STEP = /\bstep\s*(10|[1-9])\b/i;
const KEYS = {
  Tell: "tell",
  Locations: "locations",
  Evidence: "evidence",
  Verdict: "verdict",
  "Proposed fix": "fix",
  "Behaviour risk": "risk",
  Impact: "impact",
  Confidence: "confidence",
  Source: "source",
};

/** @param {string} line */
const isChoice = (line) => CHOICES.includes(line.trim());

/** @typedef {{from: number, to: number, raw: string, choice: string, note: string}} Triage */
/** @typedef {{tell: string, locations: string[], evidence: string, verdict: string, fix: string, risk: string, impact: string, confidence: string, source: string}} Fields */
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
    impact: text("impact").toLowerCase(),
    confidence: text("confidence").toLowerCase(),
    source: text("source"),
  };
  const step = Number(STEP.exec(fields.verdict)?.[1] ?? 0);
  return { id: named[1], title: named[2], section, from, to, fields, step, triage };
}

/** An audit is triageable once it holds a tell section or a chosen heading. Only the chosen tells yield
 * findings; null chooses every tell section present. An unchosen tell section reads as prose.
 * @param {string} text @param {string[] | null} [chosen] */
export function parseAudit(text, chosen = null) {
  const { title, preamble, sections } = sliceSections(text);
  const lines = text.split("\n");
  const same = (a) => (b) => a.toLowerCase() === b.toLowerCase();
  const tellOf = (name) => TELLS.find(same(name));
  const picked = chosen ?? [...new Set(sections.map((section) => tellOf(section.name)).filter(Boolean))];
  const chosenOf = (name) => picked.find(same(name));

  /** @type {Finding[]} */
  const findings = [];
  /** @type {Map<string, {name: string, from: number, to: number, count: number}>} */
  const found = new Map();
  const judged = new Set();
  for (const section of sections) {
    const tell = chosenOf(section.name);
    if (!tell || found.has(tell)) continue;
    const heads = section.nodes.filter((node) => node.type === "heading" && node.depth === 3);
    let count = 0;
    heads.forEach((heading, at) => {
      const named = FINDING.exec(plainText(heading).trim());
      if (!named) return;
      const to = heads[at + 1] ? line(heads[at + 1]) - 1 : section.to;
      findings.push(readFinding(lines, line(heading), to, tell, named));
      count++;
    });
    found.set(tell, { name: tell, from: section.from, to: section.to, count });
    judged.add(section);
  }

  return {
    title,
    preamble,
    isAudit: sections.some((section) => tellOf(section.name) || chosenOf(section.name)),
    findings,
    tells: picked.map((name) => found.get(name) ?? { name, from: 0, to: 0, count: 0 }),
    sections: sections
      .filter((section) => !judged.has(section))
      .map(({ name, from, to }) => ({ name, from, to })),
  };
}

/** Impact first, then confidence, then document order. An unrated impact sorts after every rated one.
 * @template {{fields: {impact: string, confidence: string}}} F @param {F[]} findings */
export function byImpact(findings) {
  const rank = (order, value) => {
    const at = order.indexOf(value);
    return at < 0 ? order.length : at;
  };
  return findings
    .map((finding, at) => ({ finding, at }))
    .sort(
      (a, b) =>
        rank(IMPACTS, a.finding.fields.impact) - rank(IMPACTS, b.finding.fields.impact) ||
        rank(CONFIDENCES, a.finding.fields.confidence) - rank(CONFIDENCES, b.finding.fields.confidence) ||
        a.at - b.at,
    )
    .map(({ finding }) => finding);
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
