import { criteriaSpans } from "./plan.js";
import { dealt } from "./sections.js";

/**
 * The deck is one rendered document dealt out by the source lines its blocks
 * already carry: the opening heading, the body, and the criteria the phase
 * states, each into the slide whose span holds it. A criteria span reaching
 * past its own phase claims nothing in the next one, which is bucketed by the
 * line its blocks start on.
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
 * Screen 0 is the overview; slide at index n is screen n + 1.
 * @param {{from: number, to: number}[]} slides
 * @param {number} line
 */
export function screenFor(slides, line) {
  if (!line || line < 1) return 0;
  const at = slides.findIndex((slide) => line >= slide.from && line <= slide.to);
  return at < 0 ? 0 : at + 1;
}

/**
 * A phase slide counts in phases, because that is what its heading numbers.
 * Anything else answers with its own name, which is the only honest label a
 * section has: it has no position in a sequence the document defines.
 * @param {{slides: {name: string, number: number | null}[], phases: unknown[]}} parsed
 * @param {number} screen
 */
export function counterFor(parsed, screen) {
  if (screen === 0) return "Overview";
  const slide = parsed.slides[screen - 1];
  return slide.number === null ? slide.name : `${slide.number} / ${parsed.phases.length}`;
}
