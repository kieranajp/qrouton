import { latestPerFrame } from "../frame.js";

const STAGE = "diagram-stage";
const ZOOMABLE = "zoomable";

// 100% is the size the renderer emitted, so a fitted wide diagram opens below
// it and says so; the ceiling is far enough past d2's own output that the
// smallest label on a heavily shrunk diagram is legible.
const CEILING = 8;
const STEP = Math.SQRT2;
const WHEEL_K = 0.0015;
const LINE_DELTA = 16;
const DRAG_SLOP = 4;

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
 * @param {number} scale
 * @param {number} base The fitted scale, below which there is only empty space.
 * @returns {number}
 */
export function clampScale(scale, base) {
  if (!Number.isFinite(scale)) return base;
  return Math.min(Math.max(scale, base), CEILING);
}

/**
 * Holds the content under the pointer still while the scale changes around it.
 * @param {{scale: number, tx: number, ty: number}} state
 * @param {{x: number, y: number}} at Relative to the stage's origin.
 * @param {number} next
 */
export function zoomAt(state, at, next) {
  const { scale, tx, ty } = state;
  if (!(scale > 0) || !(next > 0)) return { scale, tx, ty };
  const ratio = next / scale;
  return { scale: next, tx: at.x - (at.x - tx) * ratio, ty: at.y - (at.y - ty) * ratio };
}

/**
 * @param {number} scale
 * @param {number} direction
 * @param {number} base
 */
export function stepScale(scale, direction, base) {
  return clampScale(direction > 0 ? scale * STEP : scale / STEP, base);
}

/**
 * Where a drag moves the view, clamped so neither edge of the diagram comes
 * inside the stage. Only the primary button drags, and a drag with nothing to
 * pan moves nothing.
 * @param {{tx: number, ty: number, button: number}} grab
 * @param {{x: number, y: number}} by
 * @param {{width: number, height: number}} box
 * @param {{width: number, height: number}} content
 */
export function panBy(grab, by, box, content) {
  if (grab.button !== 0) return { tx: grab.tx, ty: grab.ty };
  return {
    tx: clampTranslate(grab.tx + by.x, box.width, content.width),
    ty: clampTranslate(grab.ty + by.y, box.height, content.height),
  };
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

  const box = () => ({ width: stage.clientWidth, height: stage.clientHeight });
  const content = () => ({
    width: emitted.width * scale(),
    height: emitted.height * scale(),
  });

  const render = () => {
    const shown = content();
    svg.style.transform = `translate(${state.tx}px, ${state.ty}px) scale(${scale()})`;
    const reach = box();
    const overflows = shown.width > reach.width + 0.5 || shown.height > reach.height + 0.5;
    stage.dataset.pannable = overflows ? "true" : "false";
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

  const paint = latestPerFrame(render);

  const move = (/** @type {{x: number, y: number}} */ at, /** @type {number} */ next) => {
    const moved = zoomAt({ scale: scale(), tx: state.tx, ty: state.ty }, at, next);
    state.multiplier = moved.scale / state.base;
    state.tx = clampTranslate(moved.tx, stage.clientWidth, emitted.width * moved.scale);
    state.ty = clampTranslate(moved.ty, stage.clientHeight, emitted.height * moved.scale);
  };

  /** @type {{x: number, y: number, moved: number} | null} */
  let grab = null;
  let dragged = false;

  const down = (/** @type {PointerEvent} */ event) => {
    if (event.button !== 0) return;
    // Otherwise the browser drags the SVG content itself.
    event.preventDefault();
    grab = { x: event.clientX, y: event.clientY, moved: 0 };
    dragged = false;
    stage.setPointerCapture(event.pointerId);
    stage.dataset.dragging = "true";
  };

  const drag = (/** @type {PointerEvent} */ event) => {
    if (!grab) return;
    const by = { x: event.clientX - grab.x, y: event.clientY - grab.y };
    const panned = panBy({ tx: state.tx, ty: state.ty, button: 0 }, by, box(), content());
    state.tx = panned.tx;
    state.ty = panned.ty;
    grab = { x: event.clientX, y: event.clientY, moved: grab.moved + Math.abs(by.x) + Math.abs(by.y) };
    paint.schedule(0);
  };

  const release = (/** @type {PointerEvent} */ event) => {
    if (!grab) return;
    dragged = grab.moved > DRAG_SLOP;
    grab = null;
    delete stage.dataset.dragging;
    if (stage.hasPointerCapture(event.pointerId)) stage.releasePointerCapture(event.pointerId);
  };

  // A drag that ends over a diagram must not reach the pane's link handler.
  const clicked = (/** @type {MouseEvent} */ event) => {
    if (!dragged) return;
    dragged = false;
    event.stopPropagation();
    event.preventDefault();
  };

  // A wheel event states its delta in whatever unit the device reports in.
  const travel = (/** @type {WheelEvent} */ event) => {
    if (event.deltaMode === 1) return event.deltaY * LINE_DELTA;
    if (event.deltaMode === 2) return event.deltaY * stage.clientHeight;
    return event.deltaY;
  };

  const wheel = (/** @type {WheelEvent} */ event) => {
    // Unmodified, the pane's scroll root gets it and the document scrolls as it
    // does over prose. A pinch arrives here too, with ctrlKey already set.
    if (!event.ctrlKey && !event.metaKey) return;
    event.preventDefault();
    const box = stage.getBoundingClientRect();
    move(
      { x: event.clientX - box.left, y: event.clientY - box.top },
      clampScale(scale() * Math.exp(-travel(event) * WHEEL_K), state.base),
    );
    paint.schedule(0);
  };

  const fit = () => {
    state.multiplier = 1;
    state.tx = 0;
    state.ty = 0;
    render();
  };

  stage.addEventListener("wheel", wheel, { passive: false });
  stage.addEventListener("dblclick", fit);
  stage.addEventListener("pointerdown", down);
  stage.addEventListener("pointermove", drag);
  stage.addEventListener("pointerup", release);
  stage.addEventListener("pointercancel", release);
  stage.addEventListener("click", clicked, true);

  const view = block.ownerDocument.defaultView;
  const observer = view?.ResizeObserver ? new view.ResizeObserver(measure) : undefined;
  observer?.observe(stage);

  stages.set(block, {
    destroy() {
      observer?.disconnect();
      paint.cancel();
      stage.removeEventListener("wheel", wheel);
      stage.removeEventListener("dblclick", fit);
      stage.removeEventListener("pointerdown", down);
      stage.removeEventListener("pointermove", drag);
      stage.removeEventListener("pointerup", release);
      stage.removeEventListener("pointercancel", release);
      stage.removeEventListener("click", clicked, true);
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
