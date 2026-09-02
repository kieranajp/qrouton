import { criteriaSpans } from "./plan.js";
import { dealt } from "./sections.js";

// Criteria spanning beyond their phase never claim blocks from the next slide.
/**
 * @param {string} html
 * @param {{slides: import("./plan.js").Slide[]}} parsed
 * @param {(html: string) => {html: string, from: number, to: number}[]} deal
 * @returns {{preamble: string, slides: {opening: string, body: string, criteria: string}[]}}
 */
export function partition(html, parsed, deal = dealt) {
  const preamble = [];
  const slides = parsed.slides.map(() => ({ opening: [], body: [], criteria: [] }));
  for (const block of deal(html)) {
    const index = parsed.slides.findIndex(
      (slide) => block.from >= slide.from && block.from <= slide.to,
    );
    if (index < 0) {
      preamble.push(block.html);
      continue;
    }
    const verify = criteriaSpans(parsed.slides[index]);
    const bucket =
      block.from === parsed.slides[index].from
        ? "opening"
        : verify && block.from >= verify.from && block.to <= verify.to
          ? "criteria"
          : "body";
    slides[index][bucket].push(block.html);
  }
  return {
    preamble: preamble.join(""),
    slides: slides.map((slide) => ({
      opening: slide.opening.join(""),
      body: slide.body.join(""),
      criteria: slide.criteria.join(""),
    })),
  };
}

/**
 * @param {{from: number, to: number}[]} slides
 * @param {number} line
 */
export function screenFor(slides, line) {
  if (!line || line < 1) return 0;
  const at = slides.findIndex((slide) => line >= slide.from && line <= slide.to);
  return at < 0 ? 0 : at + 1;
}

// Non-phase slides use their names because they have no defined sequence position.
/**
 * @param {{slides: {name: string, number: number | null}[], phases: unknown[]}} parsed
 * @param {number} screen
 */
export function counterFor(parsed, screen) {
  if (screen === 0) return "Overview";
  const slide = parsed.slides[screen - 1];
  return slide.number === null ? slide.name : `${slide.number} / ${parsed.phases.length}`;
}
