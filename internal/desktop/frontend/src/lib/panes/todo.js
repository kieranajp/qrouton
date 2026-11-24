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
const KEYS = { Where: "where", Check: "check", Run: "run", Blocks: "blocks" };
const CONTAINERS = new Set(["list", "blockquote"]);

/** @typedef {"open" | "done" | "skipped"} State */
/** @typedef {{section: string, text: string, nth: number, note: string, state: State, line: number, to: number, where: string, check: string, run: string, blocks: string, prose: string[]}} Item */
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
    const { text: said, note } = split(found[2]);
    const section = sectionOf(line);
    const nth = items.filter((earlier) => earlier.section === section && earlier.text === said).length;
    /** @type {Item} */
    const item = { section, text: said, nth, note, state: stateOf(found[1]), line, to, where: "", check: "", run: "", blocks: "", prose: [] };
    for (let at = line + 1; at <= to; at++) {
      if (nested.some((span) => at >= span.from && at <= span.to)) continue;
      const source = (lines[at - 1] ?? "").trim();
      if (!source) continue;
      const field = FIELD.exec(source);
      if (field) item[KEYS[field[1]]] = field[2];
      else item.prose.push(source);
    }
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

/** The tracked pull request an item names by URL or by org/repo#n. A bare #n names no repository.
 * @template {{repo: string, number: number}} P
 * @param {{where: string, text: string}} item @param {P[] | null | undefined} pulls @returns {P | null} */
export function pullFor(item, pulls) {
  for (const value of [item.where, item.text]) {
    const named = [...value.matchAll(PULL_URL), ...value.matchAll(PULL_REF)].sort((a, b) => (a.index ?? 0) - (b.index ?? 0));
    for (const [, repo, number] of named) {
      const pull = (pulls ?? []).find((pr) => pr.repo.toLowerCase() === repo.toLowerCase() && pr.number === Number(number));
      if (pull) return pull;
    }
  }
  return null;
}

/** @param {string} value */
export const runCommand = (value) => (value.length >= 2 && value.startsWith("`") && value.endsWith("`") ? value.slice(1, -1) : value);
