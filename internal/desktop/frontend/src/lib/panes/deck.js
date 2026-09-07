import { dealt } from "./sections.js";

/** Criteria spanning beyond their section never claim the next one's blocks.
 * @param {{from: number, to: number}[]} sections In document order.
 * @param {{criteria?: (s: any) => any, deal?: (html: string) => any[]}} [how]
 * @returns {{preamble: string, sections: {opening: string, body: string, criteria: string}[]}} */
export function partition(html, sections, how = {}) {
  const { criteria, deal = dealt } = how;
  const preamble = [];
  // A section's opening heading stays apart from its body: the pane states the
  // name itself, and the heading's own line still has to be findable.
  const parts = sections.map(() => ({ opening: [], body: [], criteria: [] }));
  for (const block of deal(html)) {
    const at = sections.findIndex(
      (section) => block.from >= section.from && block.from <= section.to,
    );
    if (at < 0) {
      preamble.push(block.html);
      continue;
    }
    const verify = criteria?.(sections[at]);
    const bucket =
      block.from === sections[at].from
        ? "opening"
        : verify && block.from >= verify.from && block.to <= verify.to
          ? "criteria"
          : "body";
    parts[at][bucket].push(block.html);
  }
  return {
    preamble: preamble.join(""),
    sections: parts.map((part) => ({
      opening: part.opening.join(""),
      body: part.body.join(""),
      criteria: part.criteria.join(""),
    })),
  };
}

/** The section a line falls in, and -1 for a line in none of them.
 * @param {{from: number, to: number}[]} sections @param {number} line */
export function holding(sections, line) {
  if (!line || line < 1) return -1;
  return sections.findIndex(
    (section) => line >= section.from && line <= section.to,
  );
}

/** The span a pane marks, cut at the end of the section it opens in: a span
 * running past that says nothing about the section after it.
 * @param {{line?: number, to?: number}} doc @param {{to: number}} opened */
export function clampedSpan(doc, opened) {
  const line = doc.line ?? 0;
  const to = doc.to ?? 0;
  return { line, to: to > line ? Math.min(to, opened.to) : to };
}

/**
 * @param {{from: number, to: number}[]} slides
 * @param {number} line
 */
export function screenFor(slides, line) {
  const at = holding(slides, line);
  return at < 0 ? 0 : at + 1;
}

/** Non-phase slides use their names because they have no defined sequence position.
 * @param {{slides: {name: string, number: number | null}[], phases: unknown[]}} parsed
 * @param {number} screen */
export function counterFor(parsed, screen) {
  if (screen === 0) return "Overview";
  const slide = parsed.slides[screen - 1];
  return slide.number === null
    ? slide.name
    : `${slide.number} / ${parsed.phases.length}`;
}
