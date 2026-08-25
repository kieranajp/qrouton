const BLOCK = [
  "p",
  "h1",
  "h2",
  "h3",
  "h4",
  "h5",
  "h6",
  "li",
  "blockquote",
  "pre",
  "td",
  "th",
  ".diff-row",
  ".diff-line",
  ".title",
  ".caps",
].join(",");

const SKIP = "script, style, textarea, input, button, [aria-hidden='true']";
const MATCH = "mark[data-document-find]";

/**
 * @param {{key?: string, metaKey?: boolean, ctrlKey?: boolean, altKey?: boolean, shiftKey?: boolean}} event
 */
export function findShortcut(event) {
  return Boolean(
    event &&
      (event.metaKey || event.ctrlKey) &&
      !event.altKey &&
      !event.shiftKey &&
      event.key?.toLowerCase() === "f",
  );
}

/** @param {HTMLElement} root */
export function clearMatches(root) {
  if (!root) return;
  const parents = new Set();
  for (const mark of root.querySelectorAll(MATCH)) {
    if (mark.parentNode) parents.add(mark.parentNode);
    mark.replaceWith(...mark.childNodes);
  }
  for (const parent of parents) parent.normalize();
}

/**
 * Marks literal, case-insensitive matches without flattening the document's
 * inline markup. A block is one search unit, so a phrase can cross emphasis or
 * syntax spans but cannot join the end of one paragraph to the next.
 * @param {HTMLElement} root
 * @param {string} query
 * @returns {HTMLElement[][]}
 */
export function markMatches(root, query) {
  clearMatches(root);
  if (!root || !query) return [];

  const groups = textGroups(root);
  const matches = [];
  const pattern = new RegExp(escape(query), "giu");
  for (const nodes of groups.values()) {
    const spans = [];
    let text = "";
    for (const node of nodes) {
      const start = text.length;
      text += node.data;
      spans.push({ node, start, end: text.length });
    }
    for (const found of text.matchAll(pattern)) {
      const start = found.index;
      const end = start + found[0].length;
      matches.push(
        spans
          .filter((span) => span.end > start && span.start < end)
          .map((span) => ({
            node: span.node,
            start: Math.max(start, span.start) - span.start,
            end: Math.min(end, span.end) - span.start,
          })),
      );
    }
  }

  const wrapped = matches.map(() => []);
  const byNode = new Map();
  matches.forEach((segments, match) => {
    for (const segment of segments) {
      const entries = byNode.get(segment.node) ?? [];
      entries.push({ ...segment, match });
      byNode.set(segment.node, entries);
    }
  });

  for (const [node, segments] of byNode) {
    segments.sort((a, b) => b.start - a.start);
    for (const segment of segments) {
      const selected = node.splitText(segment.start);
      selected.splitText(segment.end - segment.start);
      const mark = root.ownerDocument.createElement("mark");
      mark.dataset.documentFind = "";
      selected.replaceWith(mark);
      mark.append(selected);
      wrapped[segment.match].push(mark);
    }
  }
  return wrapped;
}

/**
 * @param {HTMLElement[][]} matches
 * @param {number} index
 * @returns {number}
 */
export function activateMatch(matches, index) {
  for (const group of matches) {
    for (const mark of group) mark.classList.remove("current");
  }
  if (!matches.length) return -1;
  const current = ((index % matches.length) + matches.length) % matches.length;
  for (const mark of matches[current]) mark.classList.add("current");
  matches[current][0]?.scrollIntoView({ block: "center", inline: "nearest" });
  return current;
}

/** @param {HTMLElement} root */
function textGroups(root) {
  const groups = new Map();
  const showText = root.ownerDocument.defaultView?.NodeFilter.SHOW_TEXT ?? 4;
  const walker = root.ownerDocument.createTreeWalker(root, showText);
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    const text = /** @type {Text} */ (node);
    if (!text.data || text.parentElement?.closest(SKIP)) continue;
    const closest = text.parentElement?.closest(BLOCK);
    const group = closest && root.contains(closest) ? closest : root;
    const nodes = groups.get(group) ?? [];
    nodes.push(text);
    groups.set(group, nodes);
  }
  return groups;
}

const escape = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
