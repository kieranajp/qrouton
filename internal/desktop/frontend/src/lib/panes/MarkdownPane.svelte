<script>
  import CapsLabel from "../core/CapsLabel.svelte";
  import CubeMark from "../core/CubeMark.svelte";
  import { openDocument } from "../docked.svelte.js";
  import { openURL } from "../wails.js";
  import { documentPath, linkKind, render } from "./markdown.js";
  import "./markdown.css";

  /** @type {{doc: {text: string, format: string, source: string}}} */
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
</script>

<article class="document">
  {#if doc.source}<CapsLabel tone="dim">{doc.source}</CapsLabel>{/if}
  {#if heading}
    <div class="title">
      <CubeMark size={18} />
      <span>{heading}</span>
    </div>
  {/if}
  <div class="markdown" use:links>{@html rendered.body}</div>
</article>

<style>
  .document {
    padding: 26px 34px;
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
