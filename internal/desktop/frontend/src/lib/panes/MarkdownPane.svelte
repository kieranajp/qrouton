<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import CubeMark from "../core/CubeMark.svelte";
  import { artifactTone } from "../artifacts.js";
  import { Call, copyText } from "../wails.js";
  import { diagrams, links } from "./actions.js";
  import { marks, render } from "./markdown.js";
  import { createViewportController, nextViewportSequence } from "./viewport.js";
  import "./markdown.css";

  const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

  /** @type {{doc: {text: string, format: string, source: string, path?: string, kind?: string, line?: number, to?: number, viewportEpoch?: number}, id: string, active?: boolean, scrollRoot?: HTMLElement, bare?: boolean, onMeasure?: (state: any) => unknown}} */
  let { doc, id, active = false, scrollRoot, bare = false, onMeasure } = $props();

  let rendered = $derived(render(doc.text));
  let heading = $derived(rendered.title || (doc.source ? doc.source.split("/").pop() : ""));
  let tone = $derived(artifactTone(doc.kind));
  let copied = $state(false);

  async function copyPath() {
    if (!doc.path) return;
    try {
      await copyText(doc.path);
      copied = true;
      setTimeout(() => (copied = false), 1200);
    } catch {}
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
          target,
          span,
          selected: params.active,
          nextSequence: () => nextViewportSequence(windowID),
          onMeasure: (state) => onMeasure?.(state),
          report: (report) =>
            Call.ByName(WINDOWS_SERVICE + ".ReportViewport", windowID, {
              epoch: doc.viewportEpoch,
              ...report,
            }).catch(() => {}),
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

{#snippet prose()}
  <div
    class="markdown"
    data-document-source={doc.source}
    use:links={doc.source}
    use:diagrams={{ id, text: doc.text }}
    use:viewport={{ id, active, scrollRoot }}>
    {@html rendered.body}
  </div>
{/snippet}

{#if bare}
  {@render prose()}
{:else}
  <article class="document">
    {#if doc.source}
      <div class="source">
        <CapsLabel tone="dim">{doc.source}</CapsLabel>
        {#if doc.path}
          <Button
            variant="ghost"
            size="sm"
            aria-label="Copy absolute path"
            title={doc.path}
            onclick={copyPath}>{copied ? "Copied" : "Copy"}</Button>
        {/if}
      </div>
    {/if}
    {#if heading}
      <div class="title">
        <CubeMark size={18} face={tone} data-artifact-kind={doc.kind ?? "NOTE"} />
        <span>{heading}</span>
      </div>
    {/if}
    {@render prose()}
  </article>
{/if}

<style>
  .document {
    padding: 26px 34px;
  }

  /* The heading and the path start where the prose does, right of the gutter. */
  .source,
  .title {
    padding-left: var(--gutter);
  }

  .source {
    display: flex;
    align-items: center;
    gap: 9px;
  }

  .source :global(.caps) {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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
