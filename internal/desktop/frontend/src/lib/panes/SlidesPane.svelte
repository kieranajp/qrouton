<script>
  import CapsLabel from "../core/CapsLabel.svelte";
  import CubeMark from "../core/CubeMark.svelte";
  import { artifactTone } from "../artifacts.js";
  import { links, viewport } from "./actions.js";
  import CopyPath from "./CopyPath.svelte";
  import { deckSlides, renderDeck, SLIDE_WIDTH } from "./slides.js";
  import { slides } from "./slides.svelte.js";
  import "./markdown.css";

  /** @type {{doc: {text: string, format: string, source: string, path?: string, kind?: string, line?: number, to?: number, viewportEpoch?: number}, id: string, active?: boolean, scrollRoot?: HTMLElement, onScroller?: (element: HTMLElement | null) => void}} */
  let { doc, id, active = false, scrollRoot, onScroller: _onScroller } = $props();

  let cards = $derived(deckSlides(doc.text));
  let sheet = $derived(renderDeck(doc.text).css);
  let heading = $derived(doc.source ? doc.source.split("/").pop() : "");

  const deck = slides({ cards: () => cards });

  const port = viewport({
    span: () => ({ line: doc.line ?? 0, to: doc.to ?? 0 }),
    epoch: () => doc.viewportEpoch,
    onMeasure: (state) => deck.measure(state),
  });

  // A slide is laid out in Marp's fixed pixel box, so the stack measures itself
  // and hands the cards the factor that brings that box to pane width.
  let stack = $state();
  let scale = $state(1);
  $effect(() => {
    if (!stack) return;
    const observer = new ResizeObserver(([entry]) => {
      const width = entry.contentRect.width;
      if (width > 0) scale = width / SLIDE_WIDTH;
    });
    observer.observe(stack);
    return () => observer.disconnect();
  });
</script>

<svelte:head>
  {@html `<style>${sheet}</style>`}
</svelte:head>

<article class="deck">
  <div class="source">
    <CubeMark size={18} face={artifactTone(doc.kind)} data-artifact-kind={doc.kind ?? "NOTE"} />
    <span class="title">{heading}</span>
    {#if doc.source}
      <CapsLabel tone="dim">{doc.source}</CapsLabel>
    {/if}
    <CopyPath path={doc.path} />
    <span class="counter">{Math.min(deck.current + 1, cards.length)} / {cards.length}</span>
  </div>
  <div
    class="stack"
    bind:this={stack}
    style="--slide-scale: {scale}"
    use:links={doc.source}
    use:port={{ id, active, scrollRoot }}>
    {#each cards as card, index (index)}
      <div
        class="card"
        data-line={card.line || undefined}
        data-line-end={card.lineEnd || undefined}>
        <div class="frame">
          <div class="marpit">{@html card.html}</div>
        </div>
        {#if card.notes}
          <div class="notes markdown">{@html card.notes}</div>
        {/if}
      </div>
    {/each}
  </div>
</article>

<style>
  .deck {
    padding: 26px 34px;
  }

  .source {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 18px;
  }

  .title {
    font: var(--display-sm);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  .source :global(.caps) {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .counter {
    margin-left: auto;
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .stack {
    display: flex;
    flex-direction: column;
    gap: 34px;
  }

  /* The frame holds the aspect in flow while the slide inside keeps Marp's
     pixel box, so a note below a card never changes the card's height. */
  .frame {
    position: relative;
    width: 100%;
    aspect-ratio: 16 / 9;
    overflow: hidden;
    /* An outline rather than a border: the slide is scaled to the frame's own
       width, and a border would take two pixels of it away. */
    outline: var(--border-width) solid var(--border-subtle);
    outline-offset: calc(-1 * var(--border-width));
  }

  .card:global(.marked) .frame {
    outline-color: var(--border-accent);
  }

  .frame :global(.marpit) {
    position: absolute;
    top: 0;
    left: 0;
    width: 1280px;
    height: 720px;
    transform: scale(var(--slide-scale, 1));
    transform-origin: top left;
  }

  .notes {
    margin-top: 12px;
    font: var(--machine-sm);
    color: var(--text-muted);
  }
</style>
