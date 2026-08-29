import { flatten, sliceSections, walk } from "./sections.js";

/**
 * The one place the phase convention is written down: which slides are phases,
 * and what each is numbered and called. Everything downstream consumes slides,
 * so replacing the convention is this function and the planning prompt.
 * @param {string} name A slide's heading text.
 * @returns {{number: number, name: string} | null}
 */
function namesPhase(name) {
  const match = /^Phase\s+(\d+)\s*[—–:-]\s*(\S.*?)\s*$/.exec(name);
  return match ? { number: Number(match[1]), name: match[2] } : null;
}

const CRITERIA_HEADING = "verify";

// GFM only recognises `[ ]` and `[x]`, so a check a plan struck through with
// another marker parses as prose. It is still a check, and still unmet.
const OTHER_MARKER = /^\[[^\]\s]\]\s+/;

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

/** @returns {"met" | "working" | "not-started"} */
function stateOf(met, total) {
  if (total > 0 && met === total) return "met";
  return met > 0 ? "working" : "not-started";
}

/**
 * A slide is one screen of a plan. Those whose heading names a phase carry a
 * number and a meter; the rest are sections and carry neither.
 * @typedef {{text: string, met: boolean, group: number}} Criterion
 * @typedef {{screen: number, name: string, number: number | null,
 *   from: number, to: number, criteria: Criterion[], met: number, total: number,
 *   state: "met" | "working" | "not-started" | null,
 *   verify: {from: number, to: number} | null}} Slide
 */

/**
 * Reads a plan document as an overview and the slides beneath it. A document
 * with no second-level heading comes back with none, which is the signal to
 * render it plainly.
 * @param {string} text
 * @returns {{title: string, preamble: {from: number, to: number},
 *   slides: Slide[], phases: Slide[]}}
 */
export function parsePlan(text) {
  const { title, preamble, sections } = sliceSections(text);

  const slides = sections.map((section, at) => {
    const phase = namesPhase(section.name);
    const span = {
      screen: at + 1,
      name: phase ? phase.name : section.name,
      number: phase ? phase.number : null,
      from: section.from,
      to: section.to,
    };
    if (!phase) {
      return { ...span, criteria: [], met: 0, total: 0, state: null, verify: null };
    }
    const { heading, nodes } = criteriaSection(section.nodes);
    const { criteria, end } = readCriteria(nodes);
    const met = criteria.filter((criterion) => criterion.met).length;
    return {
      ...span,
      criteria,
      met,
      total: criteria.length,
      state: stateOf(met, criteria.length),
      verify: heading
        ? {
            from: heading.position?.start?.line ?? 0,
            to: Math.max(end, heading.position?.end?.line ?? 0),
          }
        : null,
    };
  });

  return {
    title,
    preamble,
    slides,
    phases: slides.filter((slide) => slide.number !== null),
  };
}

/**
 * The source lines the criteria heading and its list occupy, so a renderer can
 * lift exactly those out of the phase body.
 * @param {Slide | undefined} slide
 */
export function criteriaSpans(slide) {
  return slide?.verify ?? null;
}
