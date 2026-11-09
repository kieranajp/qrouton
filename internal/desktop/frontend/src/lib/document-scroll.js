/** Nothing in the app scrolls the document, so any offset is a stray reveal. */
/** @param {Window & typeof globalThis} [view] */
export function pinDocumentScroll(view = window) {
  const reset = () => {
    const el = view.document.scrollingElement;
    if (!el) return;
    if (el.scrollTop !== 0) el.scrollTop = 0;
    if (el.scrollLeft !== 0) el.scrollLeft = 0;
  };
  view.addEventListener("scroll", reset, { passive: true });
  reset();
  return () => view.removeEventListener("scroll", reset);
}
