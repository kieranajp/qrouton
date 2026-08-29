import { openDocument } from "../docked.svelte.js";
import { Call, Events, openURL } from "../wails.js";
import { apply as applyDiagrams, teardown as teardownDiagrams } from "./diagrams.js";
import { documentPath, linkKind } from "./markdown.js";

const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

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
  const off = Events.On("window:diagram:" + id, (event) => applyDiagrams(body, [event.data]));
  // Rendered markup does not survive a content push, so the fences are asked
  // for again whenever the text behind them changes.
  const draw = () =>
    Call.ByName(WINDOWS_SERVICE + ".RenderDiagrams", id)
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
