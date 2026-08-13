<script>
  import CapsLabel from "../core/CapsLabel.svelte";
  import CubeMark from "../core/CubeMark.svelte";
  import { openDocument } from "../docked.svelte.js";
  import { Call, openURL } from "../wails.js";
  import { documentPath, linkKind, marks, render } from "./markdown.js";
  import { createViewportController, nextViewportSequence } from "./viewport.js";
  import "./markdown.css";

  const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

  /** @type {{doc: {text: string, format: string, source: string, line?: number, to?: number}, id: string, active?: boolean, scrollRoot?: HTMLElement}} */
  let { doc, id, active = false, scrollRoot } = $props();

  let rendered = $derived(render(doc.text));
  let heading = $derived(rendered.title || (doc.source ? doc.source.split("/").pop() : ""));

  /** @param {HTMLElement} body */
  function links(body) {
    /** @param {MouseEvent} event */
    const click = (event) => {
      const anchor = /** @type {HTMLElement} */ (event.target)?.closest("a");
      if (!anchor) return;
      const href = anchor.getAttribute("href");
      // Following it would replace the app with a file the webview cannot draw.
      event.preventDefault();
      if (linkKind(href) === "document") {
        openDocument(documentPath(href ?? "", doc.source)).catch(() => {});
      } else if (linkKind(href) === "external") {
        openURL(href ?? "");
      }
    };
    body.addEventListener("click", click);
    return { destroy: () => body.removeEventListener("click", click) };
  }

  /** @param {HTMLElement} body */
  function viewport(body, initial) {
    const blocks = [.../** @type {NodeListOf<HTMLElement>} */ (body.querySelectorAll("[data-line]"))];
    const span = { line: doc.line ?? 0, to: doc.to ?? 0 };
    const { marked, at } = marks(
      blocks.map((el) => ({ line: Number(el.dataset.line), end: Number(el.dataset.lineEnd) })),
      span,
    );
    for (const index of marked) blocks[index].classList.add("marked");
    const target = blocks[at];
    let controller;
    let root;
    let windowID;
    const apply = (params) => {
      if (!params.scrollRoot) return;
      if (!controller || root !== params.scrollRoot || windowID !== params.id) {
        controller?.destroy();
        root = params.scrollRoot;
        windowID = params.id;
        controller = createViewportController({
          root,
          content: body,
          blocks,
          target,
          span,
          selected: params.active,
          nextSequence: () => nextViewportSequence(windowID),
          report: (report) =>
            Call.ByName(WINDOWS_SERVICE + ".ReportViewport", windowID, report).catch(() => {}),
        });
        return;
      }
      controller.setSelected(params.active);
    };
    apply(initial);
    return {
      update: apply,
      destroy: () => controller?.destroy(),
    };
  }
</script>

<article class="document">
  {#if doc.source}<CapsLabel tone="dim">{doc.source}</CapsLabel>{/if}
  {#if heading}
    <div class="title">
      <CubeMark size={18} />
      <span>{heading}</span>
    </div>
  {/if}
  <div class="markdown" use:links use:viewport={{ id, active, scrollRoot }}>
    {@html rendered.body}
  </div>
</article>

<style>
  .document {
    padding: 26px 34px;
  }

  /* The heading and the path start where the prose does, right of the gutter. */
  .document :global(.caps),
  .title {
    padding-left: var(--gutter);
  }

  .title {
    display: flex;
    align-items: center;
    gap: 12px;
    margin: 10px 0 18px;
    font: var(--display-sm);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }
</style>
