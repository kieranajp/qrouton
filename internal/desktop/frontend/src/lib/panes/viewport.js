/** @param {{line: number, to: number}[]} intervals */
export function normalizeIntervals(intervals) {
  const ordered = intervals.map(({ line, to }) => {
    if (!Number.isInteger(line) || !Number.isInteger(to) || line < 1 || to < line) {
      throw new TypeError(`invalid source interval ${line}-${to}`);
    }
    return { line, to };
  });
  ordered.sort((a, b) => a.line - b.line || a.to - b.to);
  const merged = [];
  for (const interval of ordered) {
    const last = merged.at(-1);
    if (last && interval.line <= last.to + 1) {
      last.to = Math.max(last.to, interval.to);
    } else {
      merged.push(interval);
    }
  }
  return merged;
}

/**
 * @param {HTMLElement} root
 * @param {HTMLElement[]} blocks
 * @param {boolean} selected
 */
export function measureViewport(root, blocks, selected) {
  if (!selected) return { available: false, selected: false, intervals: [] };
  const viewport = root?.getBoundingClientRect();
  if (!viewport || viewport.width <= 0 || viewport.height <= 0) {
    return { available: false, selected: true, intervals: [] };
  }
  const intervals = [];
  for (const block of blocks) {
    const rect = block.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0 || rect.bottom <= viewport.top || rect.top >= viewport.bottom) {
      continue;
    }
    const line = Number(block.dataset.line);
    const to = Number(block.dataset.lineEnd);
    intervals.push({ line, to });
  }
  return { available: true, selected: true, intervals: normalizeIntervals(intervals) };
}

const sequences = new Map();

export function nextViewportSequence(id) {
  const next = (sequences.get(id) ?? 0) + 1;
  sequences.set(id, next);
  return next;
}

/**
 * @param {{
 *   root: HTMLElement,
 *   content: HTMLElement,
 *   blocks?: HTMLElement[],
 *   target?: HTMLElement,
 *   span?: {line: number, to: number},
 *   selected?: boolean,
 *   report: (report: {seq: number, available: boolean, selected: boolean, intervals: {line: number, to: number}[]}) => unknown,
 *   requestFrame?: (callback: FrameRequestCallback) => number,
 *   cancelFrame?: (id: number) => void,
 *   resizeObserver?: typeof ResizeObserver,
 *   view?: Window,
 *   fonts?: FontFaceSet,
 *   nextSequence?: () => number,
 *   onMeasure?: (state: {intervals: {line: number, to: number}[]}) => unknown,
 * }} options
 */
export function createViewportController(options) {
  const {
    root,
    content,
    target,
    report,
    requestFrame = requestAnimationFrame,
    cancelFrame = cancelAnimationFrame,
    resizeObserver = globalThis.ResizeObserver,
    view = globalThis.window,
    fonts = globalThis.document?.fonts,
  } = options;
  let selected;
  let pending = 0;
  let pendingWork = "";
  let sequence = 0;
  const nextSequence = options.nextSequence ?? (() => ++sequence);
  let last = "";
  let destroyed = false;
  let achievedTarget = false;
  let targetVisible = false;

  const blocks = () =>
    options.blocks ?? [
      .../** @type {NodeListOf<HTMLElement>} */ (content.querySelectorAll("[data-line]")),
    ];
  const publish = (state) => {
    const normalized = { ...state, intervals: normalizeIntervals(state.intervals) };
    const key = JSON.stringify(normalized);
    if (key === last) return;
    last = key;
    report({ seq: nextSequence(), ...normalized });
  };
  const targetGeometry = () => {
    const viewport = root?.getBoundingClientRect();
    const rect = target?.getBoundingClientRect();
    const available = Boolean(
      viewport && viewport.width > 0 && viewport.height > 0 && rect && rect.width > 0 && rect.height > 0,
    );
    return {
      available,
      visible: Boolean(available && rect.bottom > viewport.top && rect.top < viewport.bottom),
    };
  };
  const measure = () => {
    const state = measureViewport(root, blocks(), selected);
    const geometry = targetGeometry();
    targetVisible = geometry.visible;
    achievedTarget ||= geometry.visible;
    publish(state);
    // The same measurement answers "what is on screen" for the pane itself,
    // rather than a second listener re-measuring the same blocks.
    options.onMeasure?.(state);
  };
  const run = () => {
    pending = 0;
    const work = pendingWork;
    pendingWork = "";
    if (destroyed || !selected) return;

    const geometry = targetGeometry();
    const shouldReveal =
      geometry.available &&
      (work === "activate" ||
        (work === "layout" && (!achievedTarget || (targetVisible && !geometry.visible))));
    if (shouldReveal) target.scrollIntoView({ block: "center" });
    measure();
  };
  const queue = (work) => {
    if (destroyed || !selected) return;
    if (work === "activate" || !pendingWork) pendingWork = work;
    if (!pending) pending = requestFrame(run);
  };
  const schedule = () => {
    if (pendingWork !== "activate") pendingWork = "measure";
    queue("measure");
  };
  const cancel = () => {
    if (!pending) return;
    cancelFrame(pending);
    pending = 0;
    pendingWork = "";
  };
  const scroll = schedule;
  const invalidate = () => queue("layout");

  root.addEventListener("scroll", scroll, { passive: true });
  content.addEventListener("load", invalidate, true);
  content.addEventListener("error", invalidate, true);
  view?.addEventListener("resize", invalidate);
  fonts?.addEventListener?.("loadingdone", invalidate);
  fonts?.ready?.then(invalidate);
  const observer = resizeObserver ? new resizeObserver(invalidate) : undefined;
  observer?.observe(root);
  observer?.observe(content);

  const setSelected = (next) => {
    const active = Boolean(next);
    if (active === selected) return;
    selected = active;
    achievedTarget = false;
    targetVisible = false;
    if (active) {
      queue("activate");
    } else {
      cancel();
      publish({ available: false, selected: false, intervals: [] });
    }
  };
  const destroy = () => {
    cancel();
    publish({ available: false, selected: false, intervals: [] });
    destroyed = true;
    root.removeEventListener("scroll", scroll);
    content.removeEventListener("load", invalidate, true);
    content.removeEventListener("error", invalidate, true);
    view?.removeEventListener("resize", invalidate);
    fonts?.removeEventListener?.("loadingdone", invalidate);
    observer?.disconnect();
  };

  setSelected(options.selected);
  return { setSelected, schedule, destroy };
}
