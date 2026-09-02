import { sliceSections } from "./sections.js";

const SUMMARY = "summary";

/** @typedef {{index: number, name: string, from: number, to: number}} Item */

// A document without second-level headings has no summary or items and renders plainly.
/**
 * @param {string} text
 * @returns {{title: string, preamble: {from: number, to: number}, summary: Item | null, items: Item[]}}
 */
export function parseResearch(text) {
  const { title, preamble, sections } = sliceSections(text);

  const items = sections.map((section, at) => ({
    index: at,
    name: section.name,
    from: section.from,
    to: section.to,
  }));

  // Only the opening section is the summary. One further down is a section
  // about the summary, and belongs in the accordion with everything else.
  const summary = items[0]?.name.toLowerCase() === SUMMARY ? items[0] : null;

  return { title, preamble, summary, items: summary ? items.slice(1) : items };
}
