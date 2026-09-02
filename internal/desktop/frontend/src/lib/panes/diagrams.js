import { attach, detach } from "./diagram-view.js";

const PENDING = "diagram-pending";
const DRAWN = "diagram";
const FAILED = "diagram-failed";
const NOTE = "diagram-error";

// What output carrying no size of its own is drawn at.
const EMITTED_SCALE = 0.65;

/**
 * @typedef {{line?: number, svg?: string, error?: string}} Rendered
 */

/**
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
 * @param {string | null | undefined} viewBox
 * @returns {{width: number, height: number} | null}
 */
export function naturalSize(viewBox) {
  const box = (viewBox ?? "").trim().split(/[\s,]+/).map(Number);
  if (box.length !== 4 || box.some((value) => !Number.isFinite(value))) return null;
  const [, , width, height] = box;
  return width > 0 && height > 0 ? { width, height } : null;
}

// Emitted dimensions are separate from the SVG's natural viewBox size.
/**
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
  // A percentage or an em would parse to a number meaning something else.
  if (!/^\d*\.?\d+(px)?$/.test((value ?? "").trim())) return 0;
  return Number.parseFloat(value ?? "");
}

// Diagram blocks retain their element identity while their contents change.
/**
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

// SVG markup has passed the backend's safety check before reaching this renderer.
/**
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
