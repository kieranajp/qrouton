<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import Chip from "../core/Chip.svelte";
  import CubeMark from "../core/CubeMark.svelte";
  import { untrack } from "svelte";
  import { artifactTone } from "../artifacts.js";
  import { Call, copyText } from "../wails.js";
  import { diagrams, links } from "./actions.js";
  import MarkdownPane from "./MarkdownPane.svelte";
  import { marks, render } from "./markdown.js";
  import { parseResearch } from "./research.js";
  import { dealt } from "./sections.js";
  import { createViewportController, nextViewportSequence } from "./viewport.js";
  import "./markdown.css";

  const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

  /** @type {{doc: {text: string, format: string, source: string, path?: string, kind?: string, line?: number, to?: number, viewportEpoch?: number}, id: string, active?: boolean, scrollRoot?: HTMLElement}} */
  let { doc, id, active = false, scrollRoot } = $props();

  let rendered = $derived(render(doc.text));
  let research = $derived(parseResearch(doc.text));
  let parts = $derived(partition(rendered.body, research));
  let heading = $derived(rendered.title || (doc.source ? doc.source.split("/").pop() : ""));
  let tone = $derived(artifactTone(doc.kind));
  let copied = $state(false);
  let mode = $state("research");

  // Closed is the resting state: the accordion is an index, and an index the
  // reader has to fold up again is no index at all.
  let open = $state(untrack(() => holding(research, doc.line ?? 0)));
  let epoch = untrack(() => doc.viewportEpoch);

  // A push carries the span along with the text, so only a reload — which is
  // what moves the epoch — counts as a fresh request to open an item.
  $effect(() => {
    const at = doc.viewportEpoch;
    const asked = research;
    untrack(() => {
      if (at === epoch) return;
      epoch = at;
      open = holding(asked, doc.line ?? 0);
    });
  });

  /** Every section in document order, which is the order their indexes run in. */
  function sectionsOf(parsed) {
    return parsed.summary ? [parsed.summary, ...parsed.items] : parsed.items;
  }

  /** The item a line falls in, if any: the one the pane opens on. */
  function holding(parsed, line) {
    const found = parsed.items.find((item) => line >= item.from && line <= item.to);
    return new Set(line > 0 && found ? [found.index] : []);
  }

  // The document is rendered whole and dealt out to the sections it was written
  // as. Each section's opening heading is kept apart from its body: the pane
  // states the name itself, and the heading's own line still has to be findable.
  function partition(html, parsed) {
    const sections = sectionsOf(parsed);
    const preamble = [];
    const bodies = sections.map(() => ({ opening: [], body: [] }));
    for (const block of dealt(html)) {
      const at = sections.findIndex(
        (section) => block.from >= section.from && block.from <= section.to,
      );
      if (at < 0) {
        preamble.push(block.html);
        continue;
      }
      bodies[at][block.from === sections[at].from ? "opening" : "body"].push(block.html);
    }
    return {
      preamble: preamble.join(""),
      sections: bodies.map((section) => ({
        opening: section.opening.join(""),
        body: section.body.join(""),
      })),
    };
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

  async function copyPath() {
    if (!doc.path) return;
    try {
      await copyText(doc.path);
      copied = true;
      setTimeout(() => (copied = false), 1200);
    } catch {}
  }

  // A span running past the end of the item it opens in says nothing about the
  // one after it, so the pane neither marks that part nor scrolls to it.
  function requested() {
    const line = doc.line ?? 0;
    const to = doc.to ?? 0;
    const opened =
      sectionsOf(research).find((section) => line >= section.from && line <= section.to) ??
      research.preamble;
    return { line, to: to > line ? Math.min(to, opened.to) : to };
  }

  /** @param {HTMLElement} sheet */
  function viewport(sheet, initial) {
    const blocks = [.../** @type {NodeListOf<HTMLElement>} */ (sheet.querySelectorAll("[data-line]"))];
    const span = requested();
    const { marked, at } = marks(
      blocks.map((el) => ({ line: Number(el.dataset.line), end: Number(el.dataset.lineEnd) })),
      span,
    );
    for (const index of marked) blocks[index].classList.add("marked");
    const target = blocks[at];
    let controller;
    let root;
    let windowID;
    let shown;
    const apply = (params) => {
      if (!params.scrollRoot) return;
      if (!controller || root !== params.scrollRoot || windowID !== params.id) {
        controller?.destroy();
        root = params.scrollRoot;
        windowID = params.id;
        shown = params.shown;
        controller = createViewportController({
          root,
          content: sheet,
          target,
          span,
          selected: params.active,
          nextSequence: () => nextViewportSequence(windowID),
          report: (report) =>
            Call.ByName(WINDOWS_SERVICE + ".ReportViewport", windowID, {
              epoch: doc.viewportEpoch,
              ...report,
            }).catch(() => {}),
        });
        return;
      }
      controller.setSelected(params.active);
      // Opening an item changes what can be measured, never where to scroll.
      if (shown !== params.shown) {
        shown = params.shown;
        controller.schedule();
      }
    };
    apply(initial);
    return {
      update: apply,
      destroy: () => controller?.destroy(),
    };
  }
</script>

{#if !research.summary && research.items.length === 0}
  <MarkdownPane {doc} {id} {active} {scrollRoot} />
{:else}
  <article class="document research">
    <div class="head">
      <CubeMark size={18} face={tone} data-artifact-kind={doc.kind ?? "NOTE"} />
      <Chip>{doc.kind ?? "RESEARCH"}</Chip>
      {#if doc.source}
        <CapsLabel tone="dim">{doc.source}</CapsLabel>
      {/if}
      {#if doc.path}
        <Button
          variant="ghost"
          size="sm"
          aria-label="Copy absolute path"
          title={doc.path}
          onclick={copyPath}>{copied ? "Copied" : "Copy"}</Button>
      {/if}
    </div>
    {#if mode === "document"}
      <div class="reading">
        <!-- The renderer lifts the opening heading out of the body, so the
             document view states the research's name itself. -->
        <h1 class="display-lg">{research.title || heading}</h1>
        <MarkdownPane {doc} {id} {active} {scrollRoot} bare />
      </div>
    {:else}
      <div
        class="sheet"
        data-document-source={doc.source}
        use:links={doc.source}
        use:diagrams={{ id, text: doc.text }}
        use:viewport={{ id, active, scrollRoot, shown: [...open].sort().join(",") }}>
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
    <footer class="footer">
      <div class="controls">
        {#if mode === "research"}
          <div class="steps">
            <Button variant="ghost" size="sm" onclick={() => showAll(true)}>Expand all</Button>
            <Button variant="ghost" size="sm" onclick={() => showAll(false)}>Collapse all</Button>
          </div>
        {/if}
        <div class="modes">
          <Button
            variant={mode === "research" ? "outline" : "ghost"}
            size="sm"
            aria-pressed={mode === "research"}
            onclick={() => (mode = "research")}>Research</Button>
          <Button
            variant={mode === "document" ? "outline" : "ghost"}
            size="sm"
            aria-pressed={mode === "document"}
            onclick={() => (mode = "document")}>Document</Button>
        </div>
        <span class="counter">
          {research.items.length}
          {research.items.length === 1 ? "section" : "sections"}
        </span>
      </div>
    </footer>
  </article>
{/if}

<style>
  .document {
    --pane-pad: 34px;
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
  }

  /* The footer spans the pane, so the padding belongs to what it frames. */
  .sheet,
  .reading {
    flex: 1;
    padding: 0 var(--pane-pad) 26px;
  }

  /* Aligned with the body's text column rather than the pane edge, so the
     mark, the chip and the path sit over the sheet's own left margin. */
  .head {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 26px var(--pane-pad) 20px calc(var(--pane-pad) + var(--gutter));
  }

  .head :global(.caps) {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .display-lg {
    margin: 4px 0 18px;
    padding-left: var(--gutter);
    font: var(--display-lg);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
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

  /* Held on the pane's floor however tall the sheet is. */
  .footer {
    position: sticky;
    bottom: 0;
    margin-top: auto;
    background: var(--surface-chrome);
    border-top: var(--border-width) solid var(--border-subtle);
  }

  .controls {
    display: flex;
    align-items: center;
    gap: 14px;
    min-height: var(--h-footer);
    padding: 0 var(--pane-pad);
  }

  .steps,
  .modes {
    display: flex;
    flex: none;
    gap: 6px;
  }

  .counter {
    flex: 1 1 0;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: right;
    font: var(--machine-sm);
    color: var(--text-muted);
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

    .controls {
      flex-wrap: wrap;
      row-gap: 10px;
      padding-top: 8px;
      padding-bottom: 8px;
    }
  }
</style>
