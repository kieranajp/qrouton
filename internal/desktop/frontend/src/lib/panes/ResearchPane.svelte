<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import ArtifactTag from "../core/ArtifactTag.svelte";
  import { untrack } from "svelte";
  import ArtifactPane from "./ArtifactPane.svelte";
  import { diagrams, links, viewport } from "./actions.js";
  import { clampedSpan, holding, partition } from "./deck.js";
  import MarkdownPane from "./MarkdownPane.svelte";
  import { render } from "./markdown.js";
  import { reader, scrolls } from "./reader.svelte.js";
  import { parseResearch } from "./research.js";
  import "./markdown.css";

  /** @type {{doc: {text: string, format: string, source: string, path?: string, kind?: string, line?: number, to?: number, viewportEpoch?: number}, id: string, active?: boolean, scrollRoot?: HTMLElement, onScroller?: (element: HTMLElement | null) => void}} */
  let { doc, id, active = false, scrollRoot, onScroller } = $props();

  let rendered = $derived(render(doc.text));
  let research = $derived(parseResearch(doc.text));
  let sections = $derived(sectionsOf(research));
  let parts = $derived(partition(rendered.body, sections));
  let heading = $derived(rendered.title || (doc.source ? doc.source.split("/").pop() : ""));
  let structured = $derived(Boolean(research.summary) || research.items.length > 0);

  // Closed is the resting state: the accordion is an index, and an index the
  // reader has to fold up again is no index at all.
  let open = $state(untrack(() => opening(research, doc.line ?? 0)));

  const view = reader({
    structured: "research",
    doc: () => doc,
    reload: () => (open = opening(research, doc.line ?? 0)),
  });

  /** @type {HTMLElement | undefined} */
  let sheet = $state();
  /** @type {HTMLElement | undefined} */
  let reading = $state();

  scrolls({
    reading: () => view.reading,
    structured: () => sheet,
    document: () => reading,
    when: () => structured,
    onScroller: () => onScroller,
  });

  /** Every section in document order, which is the order their indexes run in. */
  function sectionsOf(parsed) {
    return parsed.summary ? [parsed.summary, ...parsed.items] : parsed.items;
  }

  /** The item a line falls in, if any: the one the pane opens on. */
  function opening(parsed, line) {
    const at = holding(parsed.items, line);
    return new Set(at < 0 ? [] : [parsed.items[at].index]);
  }

  /** @param {MouseEvent} event */
  function turn(event, index) {
    event.preventDefault();
    const next = new Set(open);
    if (next.has(index)) next.delete(index);
    else next.add(index);
    open = next;
  }

  function showAll(on) {
    open = on ? new Set(research.items.map((item) => item.index)) : new Set();
  }

  const port = viewport({
    span: () => clampedSpan(doc, sections[holding(sections, doc.line ?? 0)] ?? research.preamble),
    epoch: () => doc.viewportEpoch,
  });
</script>

{#if !structured}
  <MarkdownPane {doc} {id} {active} {scrollRoot} />
{:else}
  <ArtifactPane
    {doc}
    structured="research"
    label="Research"
    mode={view.mode}
    onMode={(next) => (view.mode = next)}>
    {#snippet tag()}
      <ArtifactTag kind={doc.kind ?? "RESEARCH"} long />
    {/snippet}
    {#snippet body()}
      {#if view.reading}
        <div class="reading" bind:this={reading}>
          <!-- The renderer lifts the opening heading out of the body, so the
               document view states the research's name itself. -->
          <h1 class="display-lg">{research.title || heading}</h1>
          <MarkdownPane {doc} {id} {active} {scrollRoot} bare />
        </div>
      {:else}
        <div
          class="sheet"
          bind:this={sheet}
          data-document-source={doc.source}
          use:links={doc.source}
          use:diagrams={{ id, text: doc.text }}
          use:port={{ id, active, scrollRoot, key: [...open].sort().join(",") }}>
          <h1 class="display-lg">{research.title || heading}</h1>
          <div class="markdown lead">{@html parts.preamble}</div>
          {#if research.summary}
            <section class="pinned">
              <CapsLabel>{research.summary.name}</CapsLabel>
              <div class="markdown lifted">{@html parts.sections[research.summary.index].opening}</div>
              <div class="markdown">{@html parts.sections[research.summary.index].body}</div>
            </section>
          {/if}
          <div class="items">
            {#each research.items as item (item.index)}
              <details class="item" data-item={item.name} open={open.has(item.index)}>
                <!-- The pane owns what is open, not the element: its own toggle
                     lands a task later, and a redraw in between undoes it.
                     Enter and Space on a summary arrive here as clicks. -->
                <summary onclick={(event) => turn(event, item.index)}>{item.name}</summary>
                <div class="markdown lifted">{@html parts.sections[item.index].opening}</div>
                <div class="markdown">{@html parts.sections[item.index].body}</div>
              </details>
            {/each}
          </div>
        </div>
      {/if}
    {/snippet}
    {#snippet controls()}
      {#if !view.reading}
        <div class="steps">
          <Button variant="ghost" size="sm" onclick={() => showAll(true)}>Expand all</Button>
          <Button variant="ghost" size="sm" onclick={() => showAll(false)}>Collapse all</Button>
        </div>
      {/if}
    {/snippet}
    {#snippet counter()}
      <span class="counter">
        {research.items.length}
        {research.items.length === 1 ? "section" : "sections"}
      </span>
    {/snippet}
  </ArtifactPane>
{/if}

<style>
  /* The footer spans the pane, so the padding belongs to what it frames. The
     scroller ends where the footer begins, so its bar stops there too rather
     than running the pane's full height with the footer floating over it. */
  .sheet,
  .reading {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0 var(--pane-pad) 26px;
  }

  .display-lg {
    margin: 4px 0 18px;
    padding-left: var(--gutter);
    text-align: center;
    font: var(--display-lg);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
    text-wrap: pretty;
  }

  .lead :global(p) {
    font: var(--machine-lg);
    font-size: 15px;
    line-height: 1.7;
    color: var(--text-secondary);
    max-width: 68ch;
  }

  .lead :global(p:first-of-type) {
    color: var(--text-primary);
  }

  .pinned {
    margin: 26px 0 8px;
  }

  .pinned > :global(.caps) {
    padding-left: var(--gutter);
  }

  .items {
    border-top: var(--border-width) solid var(--border-subtle);
  }

  .item {
    border-bottom: var(--border-width) solid var(--border-subtle);
  }

  /* The gutter is the line-number column everywhere else, so the disclosure
     mark takes it and the label starts where the prose does. */
  .item > summary {
    position: relative;
    padding: 12px 0 12px var(--gutter);
    list-style: none;
    cursor: pointer;
    font: var(--machine-md);
    color: var(--text-primary);
  }

  .item > summary::-webkit-details-marker {
    display: none;
  }

  .item > summary::before {
    content: "▸";
    position: absolute;
    left: 0;
    width: var(--gutter);
    padding-right: 1.5ch;
    box-sizing: border-box;
    font: var(--terminal-sm);
    color: var(--text-faint);
    text-align: right;
  }

  .item[open] > summary::before {
    content: "▾";
  }

  .item > summary:hover {
    background: var(--wash-selected);
  }

  .item[open] > summary {
    color: var(--accent-action);
  }

  .item > .markdown:last-child {
    padding-bottom: 14px;
  }

  /* A closed item is folded away outright rather than left to the browser's
     own hiding, which keeps a box the viewport would go on measuring. */
  .item:not([open]) > .markdown {
    display: none;
  }

  .steps {
    display: flex;
    flex: none;
    gap: 6px;
  }

  /* The pane names the section already. Hidden but still measurable: the
     viewport reports only blocks with a box, and a span may be aimed at the
     heading's own line. */
  .lifted {
    position: absolute;
    width: 1px;
    height: 1px;
    margin: 0;
    padding: 0;
    overflow: hidden;
    white-space: nowrap;
  }

  /* Narrow steps the type down. Nothing goes away. */
  @media (max-width: 420px) {
    .display-lg {
      font: var(--display-md);
    }

    .lead :global(p) {
      font: var(--machine-md);
    }

    :global(.document > .footer .controls) {
      flex-wrap: wrap;
      row-gap: 10px;
      padding-top: 8px;
      padding-bottom: 8px;
    }
  }
</style>
