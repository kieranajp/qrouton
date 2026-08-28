const PENDING = "diagram-pending";
const DRAWN = "diagram";

/**
 * One d2 fence as the workbench reports it: the source line it opens on, and
 * either the rendered SVG, the reason there is none, or neither while it is
 * still being laid out.
 * @typedef {{line?: number, svg?: string, error?: string}} Rendered
 */

/**
 * Pairs each result with the block whose source line it names. A result naming
 * a line no block carries is dropped, so a fence the page never stamped stays
 * code.
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
 * The size a d2 diagram draws at. Its outer element carries a viewBox and no
 * width, which lays it out at whatever the measure allows and takes the labels
 * down with it; at the viewBox's own size the block scrolls instead.
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
 * Draws what the workbench has rendered and marks what it is still laying out.
 * The <pre> survives the swap: it carries the gutter number, the marked
 * styling, and the line the viewport measures by.
 * @param {HTMLElement} container
 * @param {Rendered[]} results
 */
export function apply(container, results) {
  const blocks = [
    .../** @type {NodeListOf<HTMLElement>} */ (container.querySelectorAll("pre[data-line]")),
  ];
  for (const { block, result } of place(blocks, results)) {
    if (result.svg) draw(block, result.svg);
    else if (result.error) block.classList.remove(PENDING);
    else block.classList.add(PENDING);
  }
}

/**
 * Go produced this markup and vetted it, and it never touches the markdown
 * pipeline, so it is parsed as html rather than escaped as text.
 * @param {HTMLElement} block
 * @param {string} svg
 */
function draw(block, svg) {
  const holder = block.ownerDocument.createElement("div");
  holder.innerHTML = svg;
  const drawn = /** @type {SVGSVGElement | null} */ (holder.firstElementChild);
  const size = naturalSize(drawn?.getAttribute("viewBox"));
  if (drawn && size) {
    drawn.style.width = `${size.width}px`;
    drawn.style.height = `${size.height}px`;
  }
  block.classList.remove(PENDING);
  block.classList.add(DRAWN);
  block.replaceChildren(...holder.childNodes);
}
