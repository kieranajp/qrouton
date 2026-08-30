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

/**
 * Anchors in rendered markdown, resolved against the document they were written
 * in. Following one would replace the app with a file the webview cannot draw,
 * so a link to another document docks it and a link out opens a browser.
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

/**
 * The fences the workbench draws for itself. Subscribed before the call, so a
 * diagram that lands between the two is heard rather than missed; the reply
 * names every fence, so the ones still being laid out are marked as such.
 * @param {HTMLElement} body
 * @param {{id: string, text: string}} params The window a pane draws for is
 *   fixed; the text is what a redraw hangs on.
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

/**
 * What the pane can see, reported to the window it draws for, and the reveal of
 * the span that opened it. The span and its marks are read as the action
 * attaches, so a pane that swaps its body reveals the position it holds then;
 * the epoch is read per report, because a reload moves it under a body that
 * stays mounted.
 * @param {{
 *   span: () => {line: number, to: number},
 *   epoch: () => number | undefined,
 *   marking?: () => boolean,
 *   onMeasure?: (state: {intervals: {line: number, to: number}[]}) => unknown,
 * }} options
 */
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
