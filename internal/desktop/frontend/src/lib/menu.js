// Room left between the menu and the edge it would otherwise run off.
const MARGIN = 8;

/**
 * @param {{x: number, y: number}} point
 * @param {{width: number, height: number}} size
 * @param {{width: number, height: number}} viewport
 * @param {number} [scale] the UI scale the menu is drawn at, as a factor
 */
export function place(point, size, viewport, scale = 1) {
  const width = size.width * scale;
  const height = size.height * scale;
  const right = point.x + width + MARGIN > viewport.width;
  const below = point.y + height + MARGIN > viewport.height;
  return {
    left: Math.max(MARGIN, right ? viewport.width - width - MARGIN : point.x),
    top: Math.max(MARGIN, Math.min(below ? point.y - height : point.y, viewport.height - height - MARGIN)),
  };
}

/** @param {HTMLElement} node @param {{x: number, y: number}} point */
export function fitMenu(node, point) {
  const menu = /** @type {HTMLElement} */ (node.firstElementChild);
  node.style.visibility = "hidden";
  const measure = () => {
    const at = place(point, menu.getBoundingClientRect(), { width: window.innerWidth, height: window.innerHeight });
    node.style.left = `${at.left}px`;
    node.style.top = `${at.top}px`;
    node.style.visibility = "visible";
  };
  const observer = new ResizeObserver(measure);
  observer.observe(menu);
  window.addEventListener("resize", measure);
  measure();
  return {
    update(next) { point = next; measure(); },
    destroy() { observer.disconnect(); window.removeEventListener("resize", measure); },
  };
}
