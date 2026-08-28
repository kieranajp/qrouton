import { attach, detach } from "./diagram-view.js";

const PENDING = "diagram-pending";
const DRAWN = "diagram";
const FAILED = "diagram-failed";
const NOTE = "diagram-error";

// The renderer's ceiling on natural size, mirrored here only for output that
// predates it and so carries no size of its own.
const EMITTED_SCALE = 0.65;

/**
 * @typedef {{line?: number, svg?: string, error?: string}} Rendered
 */

/**
 * Pairs each result with the block whose line it names; a result naming no block is dropped.
 * @param {{dataset?: DOMStringMap}[]} blocks In document order.
 * @param {Rendered[]} results
 */
export function place(blocks, results) {
  const byLine = new Map();
  for (const block of blocks ?? []) {
    const line = Number(block?.dataset?.line);
    if (Number.isInteger(line) && !byLine.has(line)) byLine.set(line, block);
  }
  const placed = [];
  for (const result of results ?? []) {
    const block = byLine.get(Number(result?.line));
    if (block) placed.push({ block, result });
  }
  return placed;
}

/**
 * Parses an SVG viewBox's own width and height.
 * @param {string | null | undefined} viewBox
 * @returns {{width: number, height: number} | null}
 */
export function naturalSize(viewBox) {
  const box = (viewBox ?? "").trim().split(/[\s,]+/).map(Number);
  if (box.length !== 4 || box.some((value) => !Number.isFinite(value))) return null;
  const [, , width, height] = box;
  return width > 0 && height > 0 ? { width, height } : null;
}

/**
 * The size the renderer asked the diagram to draw at, which d2 writes as
 * width/height beside a viewBox left at natural size.
 * @param {{getAttribute: (name: string) => string | null} | null | undefined} svg
 * @returns {{width: number, height: number} | null}
 */
export function emittedSize(svg) {
  const width = attributeSize(svg?.getAttribute("width"));
  const height = attributeSize(svg?.getAttribute("height"));
  if (width && height) return { width, height };
  const natural = naturalSize(svg?.getAttribute("viewBox"));
  if (!natural) return null;
  return { width: natural.width * EMITTED_SCALE, height: natural.height * EMITTED_SCALE };
}

/**
 * @param {string | null | undefined} value
 * @returns {number}
 */
function attributeSize(value) {
  const size = Number.parseFloat(value ?? "");
  return Number.isFinite(size) && size > 0 ? size : 0;
}

/**
 * Draws what the workbench has rendered and marks what it is still laying out.
 * The <pre> element itself is kept, never replaced: other code holds onto that exact instance.
 * @param {HTMLElement} container
 * @param {Rendered[]} results
 */
export function apply(container, results) {
  const blocks = [
    .../** @type {NodeListOf<HTMLElement>} */ (container.querySelectorAll("pre[data-line]")),
  ];
  for (const { block, result } of place(blocks, results)) {
    if (result.svg) draw(block, result.svg);
    else if (result.error) fail(block, result.error);
    else wait(block);
  }
}

/**
 * Marks a block pending, unless it has already settled.
 * @param {HTMLElement} block
 */
function wait(block) {
  if (block.classList.contains(DRAWN) || block.classList.contains(FAILED)) return;
  block.classList.add(PENDING);
}

/**
 * States the failure reason under the block, as text — never as markup.
 * @param {HTMLElement} block
 * @param {string} message
 */
function fail(block, message) {
  detach(block);
  const stated = block.querySelector("." + NOTE);
  const note = stated ?? block.ownerDocument.createElement("span");
  note.className = NOTE;
  note.textContent = message;
  block.classList.remove(PENDING, DRAWN);
  block.classList.add(FAILED);
  if (!stated) block.append(note);
}

/**
 * Parses the SVG as markup rather than escaping it as text: it arrives pre-vetted.
 * @param {HTMLElement} block
 * @param {string} svg
 */
function draw(block, svg) {
  detach(block);
  const holder = block.ownerDocument.createElement("div");
  holder.innerHTML = svg;
  const drawn = /** @type {SVGSVGElement | null} */ (holder.firstElementChild);
  const emitted = emittedSize(drawn);
  if (drawn && emitted) {
    drawn.style.width = `${emitted.width}px`;
    drawn.style.height = `${emitted.height}px`;
  }
  block.classList.remove(PENDING, FAILED);
  block.classList.add(DRAWN);
  block.replaceChildren(...holder.childNodes);
  if (drawn && emitted) attach(block, drawn, emitted);
}

/**
 * Drops every view a container's diagrams hold, so nothing outlives the pane.
 * @param {HTMLElement} container
 */
export function teardown(container) {
  for (const block of /** @type {NodeListOf<HTMLElement>} */ (
    container.querySelectorAll("pre[data-line]")
  )) {
    detach(block);
  }
}
