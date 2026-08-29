import remarkFrontmatter from "remark-frontmatter";
import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import { unified } from "unified";

const parser = unified().use([remarkParse, remarkFrontmatter, remarkGfm]);

const FRONTMATTER = new Set(["yaml", "toml"]);

/**
 * @param {any} node
 * @param {(node: any) => void} visit In document order.
 */
export function walk(node, visit) {
  visit(node);
  for (const child of node?.children ?? []) walk(child, visit);
}

/** @param {any} node */
export function flatten(node) {
  if (typeof node?.value === "string") return node.value;
  return (node?.children ?? []).map(flatten).join("");
}

/**
 * Where one section ends and the next begins. Every second-level heading opens
 * one, whatever it is called, so a section before or after the ones a reader
 * came for is a section of its own rather than something spilled into a
 * neighbour.
 * @param {any} node An mdast node.
 * @returns {{name: string} | null}
 */
function opensSection(node) {
  if (node?.type !== "heading" || node.depth !== 2) return null;
  const name = flatten(node).trim();
  return name ? { name } : null;
}

/**
 * The document convention every workbench artifact shares: frontmatter, a title,
 * a lead, then a section per second-level heading. What each reader makes of a
 * section is its own business.
 * @typedef {{name: string, from: number, to: number, nodes: any[]}} Section
 * @param {string} text
 * @returns {{title: string, start: number, last: number,
 *   preamble: {from: number, to: number}, sections: Section[]}}
 */
export function sliceSections(text) {
  const tree = /** @type {any} */ (parser.parse(text));
  const children = tree.children ?? [];
  const last = tree.position?.end?.line ?? 1;

  const matter = children.find((node) => FRONTMATTER.has(node.type));
  const start = matter ? (matter.position?.end?.line ?? 0) + 1 : 1;
  const first = children.find((node) => !FRONTMATTER.has(node.type));
  const title = first?.type === "heading" && first.depth === 1 ? flatten(first).trim() : "";

  const openings = [];
  /** @type {any[][]} */
  const bodies = [];
  for (const node of children) {
    const opening = opensSection(node);
    if (opening) {
      openings.push({ ...opening, from: node.position?.start?.line ?? start });
      bodies.push([]);
      continue;
    }
    bodies.at(-1)?.push(node);
  }
  const sections = openings.map((opening, at) => ({
    name: opening.name,
    from: opening.from,
    to: openings[at + 1] ? openings[at + 1].from - 1 : last,
    nodes: bodies[at],
  }));

  return {
    title,
    start,
    last,
    preamble: { from: start, to: sections.length > 0 ? sections[0].from - 1 : last },
    sections,
  };
}

/** @param {Element} node */
function spanOf(node) {
  const own = Number(/** @type {HTMLElement} */ (node).dataset?.line);
  if (own > 0) {
    return { from: own, to: Number(/** @type {HTMLElement} */ (node).dataset.lineEnd) || own };
  }
  const inside = [...node.querySelectorAll("[data-line]")].map((el) => ({
    from: Number(/** @type {HTMLElement} */ (el).dataset.line),
    to: Number(/** @type {HTMLElement} */ (el).dataset.lineEnd),
  }));
  if (inside.length === 0) return null;
  return {
    from: Math.min(...inside.map((at) => at.from)),
    to: Math.max(...inside.map((at) => at.to || at.from)),
  };
}

/**
 * One rendered document cut back into the blocks it was written as, by the
 * source lines they already carry. A block the parser numbered nowhere takes
 * the range of the numbered blocks inside it, or failing that the range of the
 * block before it.
 * @param {string} html
 * @returns {{html: string, from: number, to: number}[]}
 */
export function dealt(html) {
  const holder = document.createElement("div");
  holder.innerHTML = html;
  let at = { from: 0, to: 0 };
  return [...holder.children].map((node) => {
    at = spanOf(node) ?? at;
    return { html: node.outerHTML, from: at.from, to: at.to };
  });
}
