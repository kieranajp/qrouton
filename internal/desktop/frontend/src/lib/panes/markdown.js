import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import rehypeStringify from "rehype-stringify";
import remarkFrontmatter from "remark-frontmatter";
import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import remarkRehype from "remark-rehype";
import remarkSugarHigh from "@sugar-high/remark";
import { unified } from "unified";

// Without className the sanitiser deletes every highlight; style stays out. The
// line attributes are allowed everywhere because lines() stamps whichever block
// element the source line opened.
const SCHEMA = {
  ...defaultSchema,
  attributes: {
    ...defaultSchema.attributes,
    "*": [...(defaultSchema.attributes?.["*"] ?? []), "dataLine", "dataLineEnd"],
    span: [...(defaultSchema.attributes?.span ?? []), "className"],
    code: [...(defaultSchema.attributes?.code ?? []), "className"],
    pre: [...(defaultSchema.attributes?.pre ?? []), "className"],
    input: [...(defaultSchema.attributes?.input ?? []), "type", "checked", "disabled"],
  },
  clobberPrefix: "doc-",
};

// remark-sugar-high swaps every fenced block for markup it builds itself, which
// arrives without the position the parser gave the fence. The swap keeps
// document order and produces one <pre> per fence, so the fences' positions are
// collected before it and handed back after it, by order of appearance.
function fences() {
  return (tree, file) => {
    const at = [];
    walk(tree, (node) => {
      if (node.type === "code") at.push(node.position);
    });
    file.data.fences = at;
  };
}

function restore() {
  return (tree, file) => {
    const at = /** @type {any[]} */ (file.data.fences ?? []);
    let seen = 0;
    walk(tree, (node) => {
      if (node.type !== "element" || node.tagName !== "pre") return;
      node.position ??= at[seen];
      seen++;
    });
  };
}

/**
 * @param {any} node
 * @param {(node: any) => void} visit In document order.
 */
function walk(node, visit) {
  visit(node);
  for (const child of node.children ?? []) walk(child, visit);
}

// A rendered document is reflowed prose, so a source line belongs to a block
// rather than to a visual row: the line the block opens on, and the line it ends
// on, so a pane can tell which blocks a range covers. A list defers to its
// items, since a numbered list container would only repeat its first item.
const DEFERRING = new Set(["ul", "ol"]);
const ITEM = "li";

function lines() {
  return (tree) => stamp(tree.children ?? [], true);
}

function stamp(nodes, top) {
  for (const node of nodes) {
    if (node.type !== "element") continue;
    const defers = DEFERRING.has(node.tagName);
    const item = node.tagName === ITEM;
    if ((top || item) && !defers) number(node);
    if (defers || item) stamp(node.children ?? [], false);
  }
}

// A node the parser placed nowhere — GFM's task-list checkbox, say — is left
// unnumbered rather than borrowing a neighbour's line.
function number(node) {
  const at = node.position;
  if (!at?.start?.line) return;
  node.properties = {
    ...node.properties,
    dataLine: at.start.line,
    dataLineEnd: at.end?.line ?? at.start.line,
  };
}

function title() {
  return (tree, file) => {
    const at = tree.children.findIndex((node) => node.type !== "yaml" && node.type !== "toml");
    const first = tree.children[at];
    if (at < 0 || first.type !== "heading" || first.depth !== 1) return;
    file.data.title = text(first);
    tree.children.splice(at, 1);
  };
}

function text(node) {
  if (typeof node.value === "string") return node.value;
  return (node.children ?? []).map(text).join("");
}

// lines runs before the sanitiser, which rebuilds nodes and drops the parser's
// positions along the way.
const pipeline = unified().use([
  remarkParse,
  remarkFrontmatter,
  remarkGfm,
  fences,
  remarkSugarHigh,
  title,
  remarkRehype,
  restore,
  lines,
  [rehypeSanitize, SCHEMA],
  rehypeStringify,
]);

/**
 * @param {string} markdown
 * @returns {{title: string, body: string}}
 */
export function render(markdown) {
  const file = pipeline.processSync(markdown);
  return { title: typeof file.data.title === "string" ? file.data.title : "", body: String(file) };
}

// A span between blocks marks nothing and scrolls to the following block.
/**
 * @param {{line: number, end: number}[]} blocks In document order.
 * @param {{line: number, to: number}} span
 * @returns {{marked: number[], at: number}} at is -1 when the span reaches nothing.
 */
export function marks(blocks, span) {
  const first = span?.line ?? 0;
  if (first < 1) return { marked: [], at: -1 };
  const last = span.to >= first ? span.to : first;
  const marked = [];
  let after = -1;
  blocks.forEach((block, at) => {
    if (block.line <= last && block.end >= first) marked.push(at);
    else if (after < 0 && block.line > last) after = at;
  });
  return { marked, at: marked.length > 0 ? marked[0] : after };
}

/**
 * @param {string | null} href
 * @returns {"document" | "external" | "none"}
 */
export function linkKind(href) {
  if (!href || href.startsWith("#")) return "none";
  if (/^[a-z][a-z0-9+.-]*:/i.test(href)) return /^https?:/i.test(href) ? "external" : "none";
  return /\.(md|markdown)($|[?#])/i.test(href) ? "document" : "none";
}

/**
 * @param {string} href
 * @param {string} source Both are relative to the session root.
 * @returns {string}
 */
export function documentPath(href, source) {
  const target = href.replace(/[?#].*$/, "");
  if (target.startsWith("/")) return target.slice(1);
  const parts = source.split("/").slice(0, -1);
  for (const part of target.split("/")) {
    if (part === "." || part === "") continue;
    if (part === "..") parts.pop();
    else parts.push(part);
  }
  return parts.join("/");
}
