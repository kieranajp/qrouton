import {
  WINDOW_DIAGRAM_EVENT,
  WINDOWS_RENDER_DIAGRAMS,
  WINDOWS_REPORT_VIEWPORT,
} from "../bridge/generated.js";
import { mount, unmount } from "svelte";
import { openDocument } from "../docked.svelte.js";
import { Call, Events, openURL } from "../wails.js";
import { apply as applyDiagrams, teardown as teardownDiagrams } from "./diagrams.js";
import { documentPath, linkKind, marks } from "./markdown.js";
import { createViewportController, nextViewportSequence } from "./viewport.js";
import { createDiagramStream } from "./diagram-stream.js";
import { EXPAND } from "./diagram-view.js";
import ImageLightbox from "./ImageLightbox.svelte";

/** Document links dock inside the workbench; external links open in a browser.
 * @param {HTMLElement} body
 * @param {string} source */
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

/** Events subscribe before the initial call so no completed diagram is missed.
 * @param {HTMLElement} body
 * @param {{id: string, text: string, fit?: boolean}} params */
export function diagrams(body, { id, fit }) {
  const stream = createDiagramStream(
    (request) => Call.ByName(WINDOWS_RENDER_DIAGRAMS, id, request),
    (found) => applyDiagrams(body, found, { fit }),
  );
  const off = Events.On(WINDOW_DIAGRAM_EVENT + id, (event) =>
    stream.receive([event.data]),
  );
  // Rendered markup does not survive a content push, so the fences are asked
  // for again whenever the text behind them changes.
  const draw = stream.draw;
  draw();
  /** @type {(() => void) | null} */
  let close = null;
  const expand = (/** @type {CustomEvent} */ event) => {
    const { svg, emitted } = event.detail;
    const shown = svg.cloneNode(true);
    shown.removeAttribute("style");
    const markup = new XMLSerializer().serializeToString(shown);
    const back = /** @type {HTMLElement | null} */ (document.activeElement);
    const lightbox = mount(ImageLightbox, {
      target: document.body,
      props: {
        image: { source: "Diagram", url: `data:image/svg+xml;charset=utf-8,${encodeURIComponent(markup)}` },
        aspect: emitted.width / emitted.height,
        onClose: () => close?.(),
      },
    });
    close = () => {
      close = null;
      unmount(lightbox);
      back?.focus();
    };
  };
  body.addEventListener(EXPAND, expand);
  return {
    update: draw,
    destroy: () => {
      close?.();
      body.removeEventListener(EXPAND, expand);
      stream.destroy();
      off();
      teardownDiagrams(body);
    },
  };
}

/** The epoch is read per report because reloads can move beneath a mounted pane.
 * @param {{span: () => {line: number, to: number}, epoch: () => number | undefined, marking?: () => boolean, onMeasure?: (state: {intervals: {line: number, to: number}[]}) => unknown}} options */
export function viewport({ span, epoch, marking, onMeasure }) {
  /**
   * @param {HTMLElement} content
   * @param {{id: string, active?: boolean, scrollRoot?: HTMLElement, key?: unknown, request?: unknown}} initial
   */
  return (content, initial) => {
    let controller;
    let root;
    let windowID;
    let key;
    let request;
    const create = (params) => {
      const blocks = [
        .../** @type {NodeListOf<HTMLElement>} */ (content.querySelectorAll("[data-line]")),
      ];
      const asked = span();
      const { marked, at } = marks(
        blocks.map((el) => ({ line: Number(el.dataset.line), end: Number(el.dataset.lineEnd) })),
        asked,
      );
      for (const block of blocks) block.classList.remove("marked");
      if (marking?.() ?? true) for (const index of marked) blocks[index].classList.add("marked");
      return createViewportController({
        root: params.scrollRoot,
        content,
        target: blocks[at],
        span: asked,
        selected: params.active,
        nextSequence: () => nextViewportSequence(params.id),
        onMeasure,
        report: (report) =>
          Call.ByName(WINDOWS_REPORT_VIEWPORT, params.id, {
            epoch: epoch(),
            ...report,
          }).catch(() => {}),
      });
    };
    const apply = (params) => {
      if (!params.scrollRoot) return;
      if (
        !controller ||
        root !== params.scrollRoot ||
        windowID !== params.id ||
        request !== params.request
      ) {
        controller?.destroy();
        root = params.scrollRoot;
        windowID = params.id;
        key = params.key;
        request = params.request;
        controller = create(params);
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
