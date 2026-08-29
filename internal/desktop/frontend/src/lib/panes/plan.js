import remarkFrontmatter from "remark-frontmatter";
import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import { unified } from "unified";

const parser = unified().use([remarkParse, remarkFrontmatter, remarkGfm]);

/**
 * The one place the phase-boundary convention is written down. Everything else
 * consumes the phases it returns, so replacing the convention is this function
 * and the planning prompt, and nothing more.
 * @param {any} node An mdast node.
 * @returns {{index: number, name: string} | null}
 */
function opensPhase(node) {
  if (node?.type !== "heading" || node.depth !== 2) return null;
  const match = /^Phase\s+(\d+)\s*[—–:-]\s*(\S.*?)\s*$/.exec(flatten(node));
  return match ? { index: Number(match[1]), name: match[2] } : null;
}

const CRITERIA_HEADING = "verify";

// GFM only recognises `[ ]` and `[x]`, so a check a plan struck through with
// another marker parses as prose. It is still a check, and still unmet.
const OTHER_MARKER = /^\[[^\]\s]\]\s+/;

const FRONTMATTER = new Set(["yaml", "toml"]);

/**
 * @param {any} node
 * @param {(node: any) => void} visit In document order.
 */
function walk(node, visit) {
  visit(node);
  for (const child of node?.children ?? []) walk(child, visit);
}

/** @param {any} node */
function flatten(node) {
  if (typeof node?.value === "string") return node.value;
  return (node?.children ?? []).map(flatten).join("");
}

/** The item's own words: a nested list is a criterion of its own, not this one's text. */
function itemText(item) {
  return (item.children ?? [])
    .filter((child) => child.type !== "list")
    .map(flatten)
    .join(" ")
    .trim();
}

function headingAt(node, depth) {
  return node.type === "heading" && node.depth <= depth;
}

/** The nodes under the phase's criteria heading, up to the next heading of equal or lower depth. */
function criteriaSection(nodes) {
  const at = nodes.findIndex(
    (node) => node.type === "heading" && node.depth === 3 && flatten(node).trim().toLowerCase() === CRITERIA_HEADING,
  );
  if (at < 0) return { heading: null, nodes: [] };
  const rest = nodes.slice(at + 1);
  const ends = rest.findIndex((node) => headingAt(node, 3));
  return { heading: nodes[at], nodes: ends < 0 ? rest : rest.slice(0, ends) };
}

function readCriteria(section) {
  const criteria = [];
  let last;
  let group = -1;
  let end = 0;
  for (const node of section) {
    walk(node, (child) => {
      if (child.type === "list") {
        if (child !== last) group++;
        last = child;
        end = Math.max(end, child.position?.end?.line ?? 0);
        return;
      }
      if (child.type !== "listItem") return;
      const text = itemText(child);
      const marked = child.checked !== null && child.checked !== undefined;
      if (!marked && !OTHER_MARKER.test(text)) return;
      criteria.push({
        text: marked ? text : text.replace(OTHER_MARKER, ""),
        met: child.checked === true,
        group: Math.max(group, 0),
      });
    });
  }
  return { criteria, end };
}

function stateOf(met, total) {
  if (total > 0 && met === total) return "met";
  return met > 0 ? "working" : "not-started";
}

/**
 * Reads a plan document as an overview and its phases. A document nothing opens
 * a phase in comes back with none, which is the signal to render it plainly.
 * @param {string} text
 */
export function parsePlan(text) {
  const tree = parser.parse(text);
  const children = tree.children ?? [];
  const last = tree.position?.end?.line ?? 1;

  const matter = children.find((node) => FRONTMATTER.has(node.type));
  const start = matter ? (matter.position?.end?.line ?? 0) + 1 : 1;
  const first = children.find((node) => !FRONTMATTER.has(node.type));
  const title = first?.type === "heading" && first.depth === 1 ? flatten(first).trim() : "";

  const phases = [];
  /** @type {any[][]} */
  const bodies = [];
  for (const node of children) {
    const opening = opensPhase(node);
    if (opening) {
      phases.push({ ...opening, from: node.position?.start?.line ?? start, to: last });
      bodies.push([]);
      continue;
    }
    bodies.at(-1)?.push(node);
  }
  phases.forEach((phase, at) => {
    const next = phases[at + 1];
    if (next) phase.to = next.from - 1;
    const { heading, nodes } = criteriaSection(bodies[at]);
    const { criteria, end } = readCriteria(nodes);
    phase.criteria = criteria;
    phase.total = criteria.length;
    phase.met = criteria.filter((criterion) => criterion.met).length;
    phase.state = stateOf(phase.met, phase.total);
    phase.verify = heading
      ? { from: heading.position?.start?.line ?? 0, to: Math.max(end, heading.position?.end?.line ?? 0) }
      : null;
  });

  return {
    title,
    preamble: { from: start, to: phases.length > 0 ? phases[0].from - 1 : last },
    phases,
  };
}

/**
 * The source lines the criteria heading and its list occupy, so a renderer can
 * lift exactly those out of the phase body.
 * @param {{verify?: {from: number, to: number} | null}} phase
 */
export function criteriaSpans(phase) {
  return phase?.verify ?? null;
}
