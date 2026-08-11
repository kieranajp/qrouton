import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import rehypeStringify from "rehype-stringify";
import remarkFrontmatter from "remark-frontmatter";
import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import remarkRehype from "remark-rehype";
import remarkSugarHigh from "@sugar-high/remark";
import { unified } from "unified";

// Without className the sanitiser deletes every highlight; style stays out.
const SCHEMA = {
  ...defaultSchema,
  attributes: {
    ...defaultSchema.attributes,
    span: [...(defaultSchema.attributes?.span ?? []), "className"],
    code: [...(defaultSchema.attributes?.code ?? []), "className"],
    pre: [...(defaultSchema.attributes?.pre ?? []), "className"],
    input: [...(defaultSchema.attributes?.input ?? []), "type", "checked", "disabled"],
  },
  clobberPrefix: "doc-",
};

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

const pipeline = unified().use([
  remarkParse,
  remarkFrontmatter,
  remarkGfm,
  remarkSugarHigh,
  title,
  remarkRehype,
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
