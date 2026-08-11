/**
 * dismissible calls close on a press outside node, or on Escape.
 * @param {HTMLElement} node
 * @param {() => void} close
 */
export function dismissible(node, close) {
  let onClose = close;
  const outside = (/** @type {PointerEvent} */ event) => {
    if (!node.contains(/** @type {Node} */ (event.target))) onClose?.();
  };
  const key = (/** @type {KeyboardEvent} */ event) => {
    if (event.key === "Escape") onClose?.();
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
