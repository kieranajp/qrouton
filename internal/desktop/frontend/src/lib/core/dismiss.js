/**
 * @param {HTMLElement} node
 * @param {() => void} close
 */
export function dismissible(node, close) {
  let onClose = close;
  const outside = (/** @type {PointerEvent} */ event) => {
    if (!node.contains(/** @type {Node} */ (event.target))) onClose();
  };
  const key = (/** @type {KeyboardEvent} */ event) => {
    if (event.key === "Escape") onClose();
  };
  window.addEventListener("pointerdown", outside);
  window.addEventListener("keydown", key);
  return {
    update: (/** @type {() => void} */ next) => (onClose = next),
    destroy: () => {
      window.removeEventListener("pointerdown", outside);
      window.removeEventListener("keydown", key);
    },
  };
}

/** A click dispatches at the common ancestor of press and release, so both ends are checked.
 * @param {HTMLElement} node
 * @param {() => void} close
 */
export function scrim(node, close) {
  let onClose = close;
  let pressed = false;
  const down = (/** @type {PointerEvent} */ event) => {
    pressed = event.target === node;
  };
  const up = (/** @type {PointerEvent} */ event) => {
    const released = pressed && event.target === node;
    pressed = false;
    if (released) onClose();
  };
  node.addEventListener("pointerdown", down);
  node.addEventListener("pointerup", up);
  return {
    update: (/** @type {() => void} */ next) => (onClose = next),
    destroy: () => {
      node.removeEventListener("pointerdown", down);
      node.removeEventListener("pointerup", up);
    },
  };
}
