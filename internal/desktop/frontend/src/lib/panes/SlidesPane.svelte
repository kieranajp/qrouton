<script>
  import { onDestroy } from "svelte";
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import CubeMark from "../core/CubeMark.svelte";
  import { artifactTone } from "../artifacts.js";
  import { diagrams, links, viewport } from "./actions.js";
  import CopyPath from "./CopyPath.svelte";
  import ReaderFooter from "./ReaderFooter.svelte";
  import ReaderPips from "./ReaderPips.svelte";
  import { present } from "./present.svelte.js";
  import { deckSlides, renderDeck, SLIDE_WIDTH } from "./slides.js";
  import { slides } from "./slides.svelte.js";
  import "./markdown.css";

  /** @type {{doc: {text: string, format: string, source: string, path?: string, kind?: string, assetToken?: string, line?: number, to?: number, viewportEpoch?: number}, id: string, active?: boolean, scrollRoot?: HTMLElement, onScroller?: (element: HTMLElement | null) => void}} */
  let { doc, id, active = false, scrollRoot, onScroller } = $props();

  let cards = $derived(deckSlides(doc.text, doc.assetToken));
  let sheet = $derived(renderDeck(doc.text).css);
  let heading = $derived(doc.source ? doc.source.split("/").pop() : "");

  /** @type {HTMLElement | undefined} */
  let preview = $state();
  const deck = slides({ cards: () => cards, body: () => preview });
  let pips = $derived(cards.map((_, index) => ({
    label: `Slide ${index + 1}`,
    color: "var(--text-faint)",
  })));
  let caption = $derived(`Slide ${Math.min(deck.current + 1, cards.length)} of ${cards.length}`);

  $effect(() => {
    onScroller?.(preview ?? null);
    return () => onScroller?.(null);
  });

  const port = viewport({
    span: () => ({ line: doc.line ?? 0, to: doc.to ?? 0 }),
    epoch: () => doc.viewportEpoch,
    onMeasure: () => deck.measure(),
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

  $effect(() => {
    if (!active) present.close(id);
  });

  // A closed tab is destroyed with the rest of the strip, so its pane never
  // sees itself deactivate.
  onDestroy(() => present.close(id));
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
  </div>
  <div class="preview" bind:this={preview} tabindex="-1">
    <div
      class="stack"
      bind:this={stack}
      style="--slide-scale: {scale}"
      use:links={doc.source}
      use:diagrams={{ id, text: doc.text, fit: true }}
      use:port={{ id, active, scrollRoot }}>
      {#each cards as card, index (index)}
        <div
          class="card"
          data-line={card.line || undefined}
          data-line-end={card.lineEnd || undefined}>
          <div class="slide-number">Slide {index + 1}</div>
          <div class="frame">
            <div class="marpit">{@html card.html}</div>
          </div>
          {#if card.notes}
            <div class="notes markdown">{@html card.notes}</div>
          {/if}
        </div>
      {/each}
    </div>
  </div>
  <ReaderFooter pips>
    {#snippet navigation()}
      <ReaderPips entries={pips} current={deck.current} onSelect={(index) => deck.show(index)} />
    {/snippet}
    {#snippet actions()}
      <Button
        variant="ghost"
        size="sm"
        disabled={cards.length === 0}
        onclick={() => present.open(id, () => cards, deck.current)}>Present</Button>
    {/snippet}
    {#snippet counter()}
      <span class="counter" title={caption}>{caption}</span>
      <div class="steps">
        <Button
          variant="ghost"
          size="sm"
          aria-label="Previous slide"
          disabled={deck.current === 0 || cards.length === 0}
          onclick={() => deck.show(deck.current - 1)}>←</Button>
        <Button
          variant="ghost"
          size="sm"
          aria-label="Next slide"
          disabled={deck.current >= cards.length - 1}
          onclick={() => deck.show(deck.current + 1)}>→</Button>
      </div>
    {/snippet}
  </ReaderFooter>
</article>

<style>
  /* Recessed below a slide's own ground, so a card always sits on something
     rather than blending into it. */
  .deck {
    --pane-pad: 34px;
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    background: var(--surface-terminal);
  }

  .source {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 26px var(--pane-pad) 18px;
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

  .preview {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0 var(--pane-pad) 26px;
    outline: none;
  }

  .stack {
    display: flex;
    flex-direction: column;
    gap: 34px;
  }

  .slide-number {
    margin-bottom: 10px;
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  /* The frame holds the aspect in flow while the slide inside keeps Marp's
     pixel box, so a note below a card never changes the card's height. */
  .frame {
    position: relative;
    width: 100%;
    aspect-ratio: 16 / 9;
    overflow: hidden;
    /* Outline, not inset, so the slide's own background can't paint over it;
       border-default so it reads against every layout, alt included. */
    outline: var(--border-width) solid var(--border-default);
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

  /* A d2 fence keeps the slide's own frame: the source shows until the SVG
     lands, and the drawing then fits the room the slide has left. */
  .frame :global(pre.diagram) {
    background: none;
    border: none;
    line-height: 0;
    padding: 0;
    text-align: center;
  }

  .frame :global(pre.diagram svg) {
    width: auto;
    height: auto;
    max-width: 100%;
    max-height: 400px;
  }

  .frame :global(pre.diagram-pending) {
    opacity: 0.55;
  }

  .frame :global(.diagram-error) {
    display: block;
    font-size: 20px;
    color: var(--state-failed);
    white-space: pre-wrap;
  }
</style>
