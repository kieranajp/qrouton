import { Marp } from "@marp-team/marp-core";
import { render } from "./markdown.js";
import { deckAssets, slideSpans } from "./slide-source.js";
import theme from "./slide-theme.css?raw";

// Marp Core turns inlineSVG on for itself, which buries every slide under an
// <svg><foreignObject> and needs the browser script to keep that layer upright
// against Safari. The cards scale themselves, so both are off and each section
// stays a direct child of div.marpit.
const marp = new Marp({ inlineSVG: false, script: false });
marp.themeSet.default = marp.themeSet.add(theme);

// A d2 fence carries the document lines it spans, which is how the backend
// names the diagram it lays out for it. Marp's own fence markup carries none.
marp.use((md) => {
  const fence = md.renderer.rules.fence;
  md.renderer.rules.fence = (tokens, index, options, env, self) => {
    const token = tokens[index];
    const language = (token.info ?? "").trim().split(/\s+/)[0];
    if (language !== "d2" || !token.map) return fence(tokens, index, options, env, self);
    const [from, to] = token.map;
    const source = md.utils.escapeHtml(token.content);
    return `<pre data-line="${from + 1}" data-line-end="${to}"><code class="language-d2">${source}</code></pre>\n`;
  };
});

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

/** Marp's sections paired with their source lines, title and notes; token keys
 * the asset route. Sections past the spans draw unmeasured rather than not at all.
 * @param {string} markdown @param {string} [token]
 * @returns {{html: string, title: string, notes: string, line: number, lineEnd: number}[]} */
export function deckSlides(markdown, token) {
  const rendered = renderDeck(deckAssets(markdown, token));
  const spans = slideSpans(markdown);
  return sectionsOf(rendered.html).map((section, index) => ({
    html: section.html,
    title: section.title,
    notes: notesOf(rendered.comments[index] ?? []),
    line: spans[index]?.line ?? 0,
    lineEnd: spans[index]?.lineEnd ?? 0,
  }));
}

function sectionsOf(html) {
  const container = parse(html).querySelector("div.marpit");
  return [...(container?.children ?? [])].map((section) => ({
    html: section.outerHTML,
    title: section.querySelector("h1, h2, h3")?.textContent?.trim() ?? "",
  }));
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
