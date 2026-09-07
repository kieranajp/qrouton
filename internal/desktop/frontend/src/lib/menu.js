// An estimate of what the menu stands, from the component's own metrics.
const ITEM = 34;
const RULE = 11;
const PADDING = 10;
// Room left between the menu and the edge it would otherwise run off.
const MARGIN = 8;

/** menuHeight is what a list of items and rules will stand. */
export const menuHeight = (items = []) =>
  items.reduce((total, item) => total + (item === "-" ? RULE : ITEM), PADDING);

/**
 * @param {{x: number, y: number}} point
 * @param {{width: number, height: number}} size
 * @param {{width: number, height: number}} viewport
 */
export function place(point, size, viewport) {
  const right = point.x + size.width + MARGIN > viewport.width;
  const below = point.y + size.height + MARGIN > viewport.height;
  return {
    left: Math.max(MARGIN, right ? viewport.width - size.width - MARGIN : point.x),
    top: Math.max(MARGIN, below ? point.y - size.height : point.y),
  };
}
