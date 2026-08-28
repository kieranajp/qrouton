const STAGE = "diagram-stage";
const ZOOMABLE = "zoomable";

/** @type {WeakMap<Element, {destroy: () => void}>} */
const stages = new WeakMap();

/**
 * The scale at which the whole diagram fits across the box, never enlarged: a
 * diagram carries its orientation in its shape, so opening it too small beats
 * opening it clipped.
 * @param {{width: number, height: number} | null | undefined} emitted
 * @param {number} box
 * @returns {number}
 */
export function fitScale(emitted, box) {
  const width = emitted?.width ?? 0;
  if (!(width > 0) || !(box > 0)) return 1;
  return Math.min(box / width, 1);
}

/**
 * Content smaller than the box is pinned flush against its leading edge, in
 * line with the prose; content larger is held so neither of its edges comes
 * inside the box.
 * @param {number} translate
 * @param {number} box
 * @param {number} content
 * @returns {number}
 */
export function clampTranslate(translate, box, content) {
  if (!Number.isFinite(translate)) return 0;
  return Math.min(0, Math.max(Math.min(0, box - content), translate));
}

/**
 * Installs the fixed-size stage the diagram is drawn inside, and opens it at
 * the scale that shows all of it. The stage's box is a function of pane width
 * and emitted size alone, so nothing the reader does to the view can move the
 * prose around it.
 * @param {HTMLElement} block
 * @param {SVGSVGElement} svg
 * @param {{width: number, height: number}} emitted
 */
export function attach(block, svg, emitted) {
  const stage = block.ownerDocument.createElement("div");
  stage.className = STAGE;
  stage.style.setProperty("--diagram-w", String(emitted.width));
  stage.style.setProperty("--diagram-h", String(emitted.height));
  svg.replaceWith(stage);
  stage.append(svg);
  block.classList.add(ZOOMABLE);

  const state = { base: 1, multiplier: 1, tx: 0, ty: 0 };
  const scale = () => state.base * state.multiplier;

  const render = () => {
    svg.style.transform = `translate(${state.tx}px, ${state.ty}px) scale(${scale()})`;
  };

  // A pane resize keeps the reader's level of detail and moves the scale the
  // fit is measured from underneath it.
  const measure = () => {
    state.base = fitScale(emitted, stage.clientWidth);
    const drawn = scale();
    state.tx = clampTranslate(state.tx, stage.clientWidth, emitted.width * drawn);
    state.ty = clampTranslate(state.ty, stage.clientHeight, emitted.height * drawn);
    render();
  };
  measure();

  const view = block.ownerDocument.defaultView;
  const observer = view?.ResizeObserver ? new view.ResizeObserver(measure) : undefined;
  observer?.observe(stage);

  stages.set(block, {
    destroy() {
      observer?.disconnect();
      stage.remove();
      block.classList.remove(ZOOMABLE);
    },
  });
}

/**
 * @param {HTMLElement} block
 */
export function detach(block) {
  const installed = stages.get(block);
  if (!installed) return;
  stages.delete(block);
  installed.destroy();
}
