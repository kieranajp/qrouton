import { Marp } from "@marp-team/marp-core";
import { render } from "./markdown.js";
import { slideSpans } from "./slide-source.js";
import theme from "./slide-theme.css?raw";

// Marp Core turns inlineSVG on for itself, which buries every slide under an
// <svg><foreignObject> and needs the browser script to keep that layer upright
// against Safari. The cards scale themselves, so both are off and each section
// stays a direct child of div.marpit.
const marp = new Marp({ inlineSVG: false, script: false });
marp.themeSet.default = marp.themeSet.add(theme);

/** The pixel box Marp lays a 16:9 slide out in, which the card scales down to
 * pane width. */
export const SLIDE_WIDTH = 1280;
export const SLIDE_HEIGHT = 720;

/** A deck's slides, stylesheet and per-slide speaker notes.
 * @param {string} markdown
 * @returns {{html: string, css: string, comments: string[][]}} */
export function renderDeck(markdown) {
  return marp.render(markdown ?? "");
}

/** Marp's sections paired with the source lines and notes belonging to each. A
 * deck whose sections and spans disagree draws the rest of its cards unmeasured.
 * @param {string} markdown
 * @returns {{html: string, notes: string, line: number, lineEnd: number}[]} */
export function deckSlides(markdown) {
  const rendered = renderDeck(markdown);
  const spans = slideSpans(markdown);
  return sectionsOf(rendered.html).map((html, index) => ({
    html,
    notes: notesOf(rendered.comments[index] ?? []),
    line: spans[index]?.line ?? 0,
    lineEnd: spans[index]?.lineEnd ?? 0,
  }));
}

function sectionsOf(html) {
  const container = parse(html).querySelector("div.marpit");
  return [...(container?.children ?? [])].map((section) => section.outerHTML);
}

// The app's own pipeline stamps every block with its line in the note, which
// would report the note's coordinates as if they were the document's.
function notesOf(comments) {
  if (comments.length === 0) return "";
  const parsed = parse(render(comments.join("\n\n")).body);
  const stamped = /** @type {NodeListOf<HTMLElement>} */ (parsed.querySelectorAll("[data-line]"));
  for (const block of stamped) {
    delete block.dataset.line;
    delete block.dataset.lineEnd;
  }
  return parsed.body.innerHTML;
}

const parse = (html) => new DOMParser().parseFromString(html, "text/html");
