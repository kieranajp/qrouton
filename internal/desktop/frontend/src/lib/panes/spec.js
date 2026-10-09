import { flatten, sliceSections } from "./sections.js";

const OPEN = "open questions";
const DECISIONS = "decisions";
const QUESTION = /^(Q\d+)\s*[—–:-]\s*(\S.*?)\s*$/;
const OPTION = /^\s*[-*+]\s+([A-Z])\.\s+(.*)$/;
const RECOMMENDED = /\s*\*\*\(\s*recommended\b:?\s*(.*?)\s*\)\*\*\s*/i;
const ANSWER = /^Answer(?:\s*\([^)]*\))?:[ \t]?(.*)$/;
const LONE_LETTER = /^[A-Z]$/;

/** @param {any} node */
const line = (node) => node.position?.start?.line ?? 0;

/** @param {any} node @param {number} depth */
const isHeading = (node, depth) => node.type === "heading" && node.depth === depth;

/** @param {string[]} lines @param {number} from @param {number} to 1-based, inclusive. */
function lastFilled(lines, from, to) {
  let at = to;
  while (at > from && !lines[at - 1]?.trim()) at--;
  return at;
}

/** @typedef {{letter: string, text: string, recommended: boolean, reason: string}} Option */
/** @typedef {{from: number, to: number, raw: string, letter: string, note: string}} Answer */
/** @typedef {{id: string, heading: string, from: number, to: number, context: string, options: Option[], answer: Answer | null}} Question */
/** @typedef {{kind: "question" | "group" | "decision", id: string, label: string, from: number, to: number, count: number, leads: string[]}} Decision */

/** A letter is picked only when it is all the Answer line holds; anything else is free text.
 * @param {string} first @param {string} rest @param {Option[]} options */
export function readAnswer(first, rest, options) {
  const letter = first.trim();
  if (LONE_LETTER.test(letter) && options.some((option) => option.letter === letter)) {
    return { letter, note: rest.trim() };
  }
  return { letter: "", note: [first, rest].join("\n").trim() };
}

/** @param {string[]} lines @param {any} heading @param {number} to @param {RegExpExecArray} named */
function readQuestion(lines, heading, to, named) {
  const from = line(heading);
  /** @type {Option[]} */
  const options = [];
  let firstOption = 0;
  let answerAt = 0;
  for (let at = from + 1; at <= to; at++) {
    const source = lines[at - 1];
    const option = OPTION.exec(source);
    if (option && !answerAt) {
      firstOption ||= at;
      const reason = RECOMMENDED.exec(option[2]);
      options.push({
        letter: option[1],
        text: (reason ? option[2].replace(RECOMMENDED, " ") : option[2]).trim(),
        recommended: Boolean(reason),
        reason: reason ? reason[1] : "",
      });
      continue;
    }
    if (!answerAt && ANSWER.test(source)) answerAt = at;
  }

  const contextEnd = (firstOption || answerAt || to + 1) - 1;
  const context = lines.slice(from, contextEnd).join("\n").trim();

  let answer = null;
  if (answerAt) {
    const end = lastFilled(lines, answerAt, to);
    const first = ANSWER.exec(lines[answerAt - 1])?.[1] ?? "";
    const rest = lines.slice(answerAt, end).join("\n");
    const raw = [first, rest].join("\n").trim();
    answer = { from: answerAt, to: end, raw, ...readAnswer(first, rest, options) };
  }

  return { id: named[1], heading: named[2], from, to, context, options, answer };
}

/** @param {string[]} lines @param {{from: number, to: number, nodes: any[]}} section */
function readQuestions(lines, section) {
  const heads = section.nodes.filter((node) => isHeading(node, 3));
  /** @type {Question[]} */
  const questions = [];
  heads.forEach((heading, at) => {
    const named = QUESTION.exec(flatten(heading).trim());
    if (!named) return;
    const to = heads[at + 1] ? line(heads[at + 1]) - 1 : section.to;
    questions.push(readQuestion(lines, heading, to, named));
  });
  return questions;
}

/** @param {{from: number, to: number, nodes: any[]}} section */
function readDecisions(section) {
  /** @type {Decision[]} */
  const pages = [];
  /** @type {Decision | null} */
  let page = null;
  for (const node of section.nodes) {
    if (isHeading(node, 3)) {
      const name = flatten(node).trim();
      const named = QUESTION.exec(name);
      page = {
        kind: named ? "question" : "group",
        id: named ? named[1] : "",
        label: named ? named[2] : name,
        from: line(node),
        to: 0,
        count: 0,
        leads: [],
      };
      pages.push(page);
      continue;
    }
    if (!page) {
      page = { kind: "decision", id: "", label: "Decisions", from: line(node), to: 0, count: 0, leads: [] };
      pages.push(page);
    }
    const lead = node.type === "paragraph" ? node.children?.[0] : null;
    if (lead?.type === "strong" && page.kind !== "question") page.leads.push(flatten(lead).trim().replace(/[.:]$/, ""));
  }
  pages.forEach((decision, at) => {
    decision.count = decision.kind === "question" ? 1 : decision.leads.length;
    decision.to = pages[at + 1] ? pages[at + 1].from - 1 : section.to;
  });
  return pages;
}

/** @param {{nodes: any[]}} section */
function isNone(section) {
  const [only, ...rest] = section.nodes;
  return rest.length === 0 && only?.type === "paragraph" && /^none\.$/i.test(flatten(only).trim());
}

/** A spec is answerable once Open questions holds questions or "None.", or Decisions holds a resolved question.
 * @param {string} text */
export function parseSpec(text) {
  const { title, preamble, sections } = sliceSections(text);
  const lines = text.split("\n");
  const named = (name) => sections.find((section) => section.name.toLowerCase() === name);
  const found = named(OPEN) ?? null;
  const decisionsSection = named(DECISIONS) ?? null;

  const asked = found ? readQuestions(lines, found) : [];
  const shaped = found !== null && (asked.length > 0 || isNone(found));
  const openSection = shaped ? found : null;
  const open = shaped ? asked : [];
  const decisions = decisionsSection ? readDecisions(decisionsSection) : [];
  const answered = open.filter((question) => question.answer?.raw).length;

  return {
    title,
    preamble,
    isSpec: shaped || decisions.some((decision) => decision.kind === "question"),
    open,
    answered,
    decisions,
    decisionCount: decisions.reduce((total, decision) => total + decision.count, 0),
    openSection: openSection && { from: openSection.from, to: openSection.to },
    decisionsSection: decisionsSection && { from: decisionsSection.from, to: decisionsSection.to },
    sections: sections
      .filter((section) => section !== openSection && section !== decisionsSection)
      .map(({ name, from, to }) => ({ name, from, to })),
  };
}

/** A draft is matched to its question by id and heading, so a renumbered question cannot take it.
 * @param {{id: string, heading: string}} question */
export const draftKey = (question) => `${question.id}\u0000${question.heading}`;

/** @param {{letter: string, note: string}} answer */
function answerLines({ letter, note }) {
  const text = note.replace(/\r\n?/g, "\n").trim();
  if (!text) return [letter ? `Answer: ${letter}` : "Answer:"];
  const body = text.split("\n");
  if (letter) return [`Answer: ${letter}`, ...body];
  return LONE_LETTER.test(body[0].trim()) ? ["Answer:", ...body] : [`Answer: ${body[0]}`, ...body.slice(1)];
}

/** Rewrites one question's answer block and leaves every other line as it was.
 * @param {string} text @param {Question} question @param {{letter: string, note: string}} answer */
export function spliceAnswer(text, question, answer) {
  const lines = text.split("\n");
  const next = answerLines(answer);
  if (question.answer) {
    lines.splice(question.answer.from - 1, question.answer.to - question.answer.from + 1, ...next);
    return lines.join("\n");
  }
  const after = lastFilled(lines, question.from, question.to);
  const blankAfter = lines[after] !== undefined && !lines[after].trim();
  lines.splice(after, 0, "", ...next, ...(blankAfter ? [] : [""]));
  return lines.join("\n");
}

/** Lays unsaved drafts back over a freshly parsed spec. A draft whose question
 * is gone, or now has another heading, is detached rather than dropped.
 * @template {{id: string, heading: string}} D
 * @param {{open: Question[]}} parsed @param {Map<string, D>} drafts */
export function reapply(parsed, drafts) {
  const keys = new Set(parsed.open.map(draftKey));
  return {
    questions: parsed.open.map((question) => ({ question, draft: drafts.get(draftKey(question)) ?? null })),
    detached: [...drafts.entries()].filter(([key]) => !keys.has(key)).map(([, draft]) => draft),
  };
}
