import {
  WINDOW_DIAGRAM_EVENT,
  WINDOWS_RENDER_DIAGRAMS,
  WINDOWS_REPORT_VIEWPORT,
} from "../bridge/generated.js";
import { openDocument } from "../docked.svelte.js";
import { Call, Events, openURL } from "../wails.js";
import { apply as applyDiagrams, teardown as teardownDiagrams } from "./diagrams.js";
import { documentPath, linkKind, marks } from "./markdown.js";
import { createViewportController, nextViewportSequence } from "./viewport.js";

// Document links dock inside the workbench; external links open in a browser.
/**
 * @param {HTMLElement} body
 * @param {string} source
 */
export function links(body, source) {
  let from = source;
  /** @param {MouseEvent} event */
  const click = (event) => {
    const anchor = /** @type {HTMLElement} */ (event.target)?.closest("a");
    if (!anchor) return;
    const href = anchor.getAttribute("href");
    event.preventDefault();
    if (linkKind(href) === "document") {
      openDocument(documentPath(href ?? "", from)).catch(() => {});
    } else if (linkKind(href) === "external") {
      openURL(href ?? "");
    }
  };
  body.addEventListener("click", click);
  return {
    update: (next) => (from = next),
    destroy: () => body.removeEventListener("click", click),
  };
}

// Events are subscribed before the initial call so no completed diagram is missed.
/**
 * @param {HTMLElement} body
 * @param {{id: string, text: string}} params
 */
export function diagrams(body, { id }) {
  const off = Events.On(WINDOW_DIAGRAM_EVENT + id, (event) => applyDiagrams(body, [event.data]));
  // Rendered markup does not survive a content push, so the fences are asked
  // for again whenever the text behind them changes.
  const draw = () =>
    Call.ByName(WINDOWS_RENDER_DIAGRAMS, id)
      .then((found) => applyDiagrams(body, found ?? []))
      .catch(() => {});
  draw();
  return {
    update: draw,
    destroy: () => {
      off();
      teardownDiagrams(body);
    },
  };
}

// The epoch is read per report because reloads can move beneath a mounted pane.
/** @param {{span: () => {line: number, to: number}, epoch: () => number | undefined, marking?: () => boolean, onMeasure?: (state: {intervals: {line: number, to: number}[]}) => unknown}} options */
export function viewport({ span, epoch, marking, onMeasure }) {
  /**
   * @param {HTMLElement} content
   * @param {{id: string, active?: boolean, scrollRoot?: HTMLElement, key?: unknown}} initial
   */
  return (content, initial) => {
    const blocks = [
      .../** @type {NodeListOf<HTMLElement>} */ (content.querySelectorAll("[data-line]")),
    ];
    const asked = span();
    const { marked, at } = marks(
      blocks.map((el) => ({ line: Number(el.dataset.line), end: Number(el.dataset.lineEnd) })),
      asked,
    );
    if (marking?.() ?? true) for (const index of marked) blocks[index].classList.add("marked");
    const target = blocks[at];
    let controller;
    let root;
    let windowID;
    let key;
    const apply = (params) => {
      if (!params.scrollRoot) return;
      if (!controller || root !== params.scrollRoot || windowID !== params.id) {
        controller?.destroy();
        root = params.scrollRoot;
        windowID = params.id;
        key = params.key;
        controller = createViewportController({
          root,
          content,
          target,
          span: asked,
          selected: params.active,
          nextSequence: () => nextViewportSequence(windowID),
          onMeasure,
          report: (report) =>
            Call.ByName(WINDOWS_REPORT_VIEWPORT, windowID, {
              epoch: epoch(),
              ...report,
            }).catch(() => {}),
        });
        return;
      }
      controller.setSelected(params.active);
      // Hiding part of the content changes what can be measured, never where to
      // scroll.
      if (key !== params.key) {
        key = params.key;
        controller.schedule();
      }
    };
    apply(initial);
    return {
      update: apply,
      destroy: () => controller?.destroy(),
    };
  };
}
