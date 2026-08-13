<script>
  import CapsLabel from "../core/CapsLabel.svelte";
  import CubeMark from "../core/CubeMark.svelte";
  import { openDocument } from "../docked.svelte.js";
  import { openURL } from "../wails.js";
  import { documentPath, linkKind, marks, render } from "./markdown.js";
  import "./markdown.css";

  /** @type {{doc: {text: string, format: string, source: string, line?: number, to?: number}}} */
  let { doc } = $props();

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

  /**
   * Marks the lines the agent pointed at and scrolls them into view. The action
   * runs once, after the rendered html is in the DOM: a document window holds a
   * snapshot of the file, so neither the text nor the span changes under it.
   * @param {HTMLElement} body
   */
  function focus(body) {
    const blocks = [.../** @type {NodeListOf<HTMLElement>} */ (body.querySelectorAll("[data-line]"))];
    const { marked, at } = marks(
      blocks.map((el) => ({ line: Number(el.dataset.line), end: Number(el.dataset.lineEnd) })),
      { line: doc.line ?? 0, to: doc.to ?? 0 },
    );
    for (const index of marked) blocks[index].classList.add("marked");
    const target = blocks[at];
    if (!target) return;
    // "center" rather than the top: a marked passage reads better with the
    // paragraph that led to it still on screen.
    const reveal = () => target.scrollIntoView({ block: "center" });
    reveal();
    // The pane's fonts are fetched rather than installed, and the whole document
    // reflows under that scroll when they land — which on a long file leaves the
    // passage tens of lines off. Nothing here animates, so revealing it a second
    // time once the text has settled is invisible.
    document.fonts.ready.then(() => requestAnimationFrame(reveal));
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
  <div class="markdown" use:links use:focus>{@html rendered.body}</div>
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
