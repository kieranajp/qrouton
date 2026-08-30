import { sliceSections } from "./sections.js";

const SUMMARY = "summary";

/**
 * An item is one section of a research document, and one row of the accordion.
 * Its index is the section's place in the document, which is how a renderer
 * matches it against the blocks the whole document deals out.
 * @typedef {{index: number, name: string, from: number, to: number}} Item
 */

/**
 * Reads a research document as a pinned summary and the items beneath it. A
 * document with no second-level heading comes back with neither, which is the
 * signal to render it plainly.
 * @param {string} text
 * @returns {{title: string, preamble: {from: number, to: number},
 *   summary: Item | null, items: Item[]}}
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
