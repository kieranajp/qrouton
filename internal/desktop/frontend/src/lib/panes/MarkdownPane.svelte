<script>
  import CapsLabel from "../core/CapsLabel.svelte";
  import CubeMark from "../core/CubeMark.svelte";
  import { artifactTone } from "../artifacts.js";
  import { diagrams, links, viewport } from "./actions.js";
  import CopyPath from "./CopyPath.svelte";
  import { render } from "./markdown.js";
  import "./markdown.css";

  /** @type {{doc: {text: string, format: string, source: string, path?: string, kind?: string, line?: number, to?: number, viewportEpoch?: number}, id: string, active?: boolean, scrollRoot?: HTMLElement, bare?: boolean, onMeasure?: (state: any) => unknown}} */
  let { doc, id, active = false, scrollRoot, bare = false, onMeasure } = $props();

  let rendered = $derived(render(doc.text));
  let heading = $derived(rendered.title || (doc.source ? doc.source.split("/").pop() : ""));
  let tone = $derived(artifactTone(doc.kind));

  const port = viewport({
    span: () => ({ line: doc.line ?? 0, to: doc.to ?? 0 }),
    epoch: () => doc.viewportEpoch,
    onMeasure: (state) => onMeasure?.(state),
  });
</script>

{#snippet prose()}
  <div
    class="markdown"
    data-document-source={doc.source}
    use:links={doc.source}
    use:diagrams={{ id, text: doc.text }}
    use:port={{ id, active, scrollRoot }}>
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
        <CopyPath path={doc.path} />
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
