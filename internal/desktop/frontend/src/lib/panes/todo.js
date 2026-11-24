import remarkFrontmatter from "remark-frontmatter";
import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import { unified } from "unified";
import { visit } from "unist-util-visit";
import { sliceSections } from "./sections.js";

const parser = unified().use([remarkParse, remarkFrontmatter, remarkGfm]);

const TODO_LINE = /^\s*(?:[-*+]|\d+[.)])\s+\[([ xX-])\][ \t]+(\S.*?)\s*$/;
const LEAD = /^\s*(?:[-*+]|\d+[.)])\s+/;
const FIELD = /^(?:[-*+]\s+)?(Where|Check|Run|Blocks):[ \t]*(.*?)\s*$/;
const NOTE = " — ";
const BOXES = { open: " ", done: "x", skipped: "-" };
const BLANK = /^\s*$/;
const KEYS = { Where: "where", Check: "check", Run: "run", Blocks: "blocks" };
const CONTAINERS = new Set(["list", "blockquote"]);

/** @typedef {"open" | "done" | "skipped"} State */
/** @typedef {{section: string, text: string, nth: number, note: string, state: State, line: number, to: number, where: string, check: string, run: string, blocks: string, prose: string}} Item */
/** @typedef {{key: string, section: string, text: string, state: State, note?: string}} Draft */

/** @param {string} box @returns {State} */
const stateOf = (box) => (box === "x" || box === "X" ? "done" : box === "-" ? "skipped" : "open");

/** @param {string} rest */
function split(rest) {
  const at = rest.lastIndexOf(NOTE);
  if (at < 0) return { text: rest, note: "" };
  return { text: rest.slice(0, at).trim(), note: rest.slice(at + NOTE.length).trim() };
}

/** The item's line and its todo match, or null when the list item is not a todo.
 * @param {string[]} lines @param {any} node */
function lead(lines, node) {
  const first = node.children?.[0];
  const line = first?.position?.start?.line;
  if (!line || CONTAINERS.has(first.type)) return null;
  const found = TODO_LINE.exec(lines[line - 1] ?? "");
  return found ? { line, found } : null;
}

/** @param {string} text */
export function parseTodo(text) {
  const { title, preamble, sections } = sliceSections(text);
  const lines = text.split("\n");
  const tree = /** @type {any} */ (parser.parse(text));
  const sectionOf = (/** @type {number} */ line) => sections.find((section) => line >= section.from && line <= section.to)?.name ?? "";

  /** @type {Item[]} */
  const items = [];
  visit(tree, "listItem", (node) => {
    const opened = lead(lines, node);
    if (!opened) return;
    const { line, found } = opened;
    const to = node.position?.end?.line ?? line;
    const nested = [];
    visit(node, "listItem", (child) => {
      if (child !== node && lead(lines, child)) nested.push({ from: child.position.start.line, to: child.position.end.line });
    });
    const code = [];
    visit(node, "code", (block) => void code.push({ from: block.position.start.line, to: block.position.end.line }));
    const { text: said, note } = split(found[2]);
    const section = sectionOf(line);
    const nth = items.filter((earlier) => earlier.section === section && earlier.text === said).length;
    /** @type {Item} */
    const item = { section, text: said, nth, note, state: stateOf(found[1]), line, to, where: "", check: "", run: "", blocks: "", prose: "" };
    const indent = new RegExp(`^ {0,${LEAD.exec(lines[line - 1])?.[0].length ?? 0}}`);
    const prose = [];
    for (let at = line + 1; at <= to; at++) {
      const source = lines[at - 1] ?? "";
      const field = code.some((span) => at >= span.from && at <= span.to) ? null : FIELD.exec(source.trim());
      if (nested.some((span) => at >= span.from && at <= span.to)) {
        if (prose.at(-1) !== "") prose.push("");
      } else if (field) item[KEYS[field[1]]] = field[2];
      else prose.push(BLANK.test(source) ? "" : source.replace(indent, ""));
    }
    item.prose = prose.join("\n").replace(/^\n+/, "").trimEnd();
    items.push(item);
  });

  return {
    title,
    items,
    sections: [{ name: "", from: preamble.from, to: preamble.to }, ...sections.map(({ name, from, to }) => ({ name, from, to }))],
    isTodo: items.length > 0,
  };
}

/** Items sharing a section and text are told apart by how many came before them.
 * @param {{section: string, text: string, nth: number}} item */
export const todoKey = (item) => `${item.section}\u0000${item.text}\u0000${item.nth}`;

/** A draft written into the text, or null once its item is gone.
 * @param {string} text @param {Draft} draft */
export function spliceDraft(text, draft) {
  const item = parseTodo(text).items.find((item) => todoKey(item) === draft.key);
  return item ? setState(text, item, draft.state, draft.note) : null;
}

/** One item's line rewritten with a new box and note; undefined keeps the note it has.
 * Null once the line no longer holds that item.
 * @param {string} text @param {Item} item @param {State} state @param {string} [note] */
export function setState(text, item, state, note) {
  const lines = text.split("\n");
  const source = lines[item.line - 1] ?? "";
  const found = TODO_LINE.exec(source);
  if (!found || split(found[2]).text !== item.text) return null;
  // A spaced em dash would split the note again on the next read.
  const kept = (note ?? item.note).replace(/\s+/g, " ").trim().replaceAll(NOTE, " – ");
  lines[item.line - 1] = `${LEAD.exec(source)?.[0] ?? "- "}[${BOXES[state]}] ${item.text}${kept ? `${NOTE}${kept}` : ""}`;
  return lines.join("\n");
}

/** Lays unsaved changes back over a freshly parsed todo list. A draft whose item is
 * gone, or has new text, is detached rather than dropped.
 * @template {Draft} D
 * @param {{items: Item[]}} parsed @param {Map<string, D>} pending */
export function reapply(parsed, pending) {
  const keys = new Set(parsed.items.map(todoKey));
  return {
    items: parsed.items.map((item) => ({ item, draft: pending.get(todoKey(item)) ?? null })),
    detached: [...pending.entries()].filter(([key]) => !keys.has(key)).map(([, draft]) => draft),
  };
}

const PULL_URL = /https:\/\/github\.com\/([\w.-]+\/[\w.-]+)\/pull\/(\d+)/gi;
const PULL_REF = /(?<![\w./-])([\w.-]+\/[\w.-]+)#(\d+)\b/g;

/** @template P @typedef {{repo: string, number: number, url: string, pull: P | null}} PullRef */

/** Each pull request an item names by URL or org/repo#n, once, and what Where says besides them. A bare #n names no repository.
 * @template {{repo: string, number: number}} P
 * @param {{where: string, text: string}} item @param {P[] | null | undefined} pulls @returns {{refs: PullRef<P>[], where: string}} */
export function pullRefs(item, pulls) {
  /** @type {PullRef<P>[]} */
  const refs = [];
  let where = item.where;
  for (const value of [item.where, item.text]) {
    const named = [...value.matchAll(PULL_URL), ...value.matchAll(PULL_REF)].sort((a, b) => (a.index ?? 0) - (b.index ?? 0));
    for (const match of named) {
      if (value === item.where) where = where.replace(match[0], " ");
      const repo = match[1];
      const number = Number(match[2]);
      if (refs.some((ref) => ref.repo.toLowerCase() === repo.toLowerCase() && ref.number === number)) continue;
      const pull = (pulls ?? []).find((pr) => pr.repo.toLowerCase() === repo.toLowerCase() && pr.number === number) ?? null;
      refs.push({ repo, number, url: `https://github.com/${repo}/pull/${number}`, pull });
    }
  }
  return { refs, where: /[\p{L}\p{N}]/u.test(where) ? where.replace(/\(\s*\)/g, " ").replace(/\s+/g, " ").trim() : "" };
}

/** What a field's copy button copies, or null when the value reads as prose. A value that is
 * one code span is a command, and so is a Run value with no code span in it.
 * @param {"run" | "check"} key @param {string} value */
export function command(key, value) {
  const root = /** @type {any} */ (parser.parse(value));
  const [first] = root.children;
  const only = root.children.length === 1 && first.type === "paragraph" && first.children.length === 1 ? first.children[0] : null;
  if (only?.type === "inlineCode") return only.value;
  let spans = false;
  visit(root, "inlineCode", () => void (spans = true));
  return key === "run" && !spans ? value : null;
}
