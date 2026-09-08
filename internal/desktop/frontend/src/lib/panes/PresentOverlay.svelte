<script>
  import { onMount } from "svelte";
  import {
    PRESENTER_CLOSE,
    PRESENTER_CLOSED_EVENT,
    PRESENTER_OPEN,
    PRESENTER_SHOW,
  } from "../bridge/generated.js";
  import Button from "../core/Button.svelte";
  import { call, Call, Events } from "../wails.js";
  import { present } from "./present.svelte.js";
  import { SLIDE_HEIGHT, SLIDE_WIDTH } from "./slides.js";

  // Space and the page keys are what a presenter remote sends.
  const STEPS = { ArrowRight: 1, PageDown: 1, " ": 1, ArrowLeft: -1, PageUp: -1 };

  let layer = $state(/** @type {HTMLElement | undefined} */ (undefined));
  let stage = $state(/** @type {HTMLElement | undefined} */ (undefined));
  let scale = $state(1);
  let notes = $state(false);
  let card = $derived(present.cards[present.current]);
  let first = $derived(present.current === 0);
  let last = $derived(present.current >= present.total - 1);

  // The layer takes the keyboard on mount and hands it back on the way out.
  onMount(() => {
    const beneath = /** @type {HTMLElement | null} */ (document.activeElement);
    layer?.focus();
    const off = Events.On(PRESENTER_CLOSED_EVENT, () => (notes = false));
    return () => {
      off?.();
      // Leaving the presentation takes the notes window with it, whether the
      // presenter pressed Escape or the deck stopped being drawn.
      call(Call.ByName(PRESENTER_CLOSE));
      beneath?.focus?.();
    };
  });

  // Both dimensions of the inset box enter the sum: a slide fitted to width
  // alone runs off the bottom of a tall window.
  $effect(() => {
    if (!stage) return;
    const observer = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect;
      if (width > 0 && height > 0) {
        scale = Math.min(width / SLIDE_WIDTH, height / SLIDE_HEIGHT);
      }
    });
    observer.observe(stage);
    return () => observer.disconnect();
  });

  // Unconditional: the workbench retains the note whether or not the second
  // window is up, and a deck edited underneath the presentation moves both.
  $effect(() => {
    call(
      Call.ByName(PRESENTER_SHOW, {
        index: present.current,
        total: present.total,
        title: card?.title ?? "",
        html: card?.notes ?? "",
      }),
    );
  });

  // The keyboard goes straight back to the layer: an arrow that disables at the
  // end of the deck would otherwise take the arrow keys down with it.
  function move(by) {
    present.step(by);
    layer?.focus();
  }

  // The stage passes presses through, so one that reaches the layer itself
  // landed on the scrim rather than on the slide or the controls. Both ends of
  // the gesture have to land there: a click is dispatched at the common
  // ancestor, so a selection dragged off the slide would otherwise leave.
  let from = false;

  function down(event) {
    from = event.target === event.currentTarget;
  }

  function up(event) {
    const scrim = from && event.target === event.currentTarget;
    from = false;
    if (scrim) present.leave();
  }

  // The keyboard belongs to this window, so the control hands it straight back
  // rather than leaving it on a button the arrows would not reach past.
  async function toggleNotes() {
    const wanted = !notes;
    notes = wanted;
    layer?.focus();
    const answer = await call(Call.ByName(wanted ? PRESENTER_OPEN : PRESENTER_CLOSE));
    if (!answer.ok) notes = !wanted;
  }

  function key(event) {
    // The presentation is exclusive, so no window listener acts on a key while
    // it is up — a panel opening behind the scrim holds the keyboard with
    // nothing legible on screen to say so.
    event.stopPropagation();
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    if (event.key === "Escape") {
      event.preventDefault();
      present.leave();
      return;
    }
    if (event.key === "Home" || event.key === "End") {
      event.preventDefault();
      present.go(event.key === "Home" ? 0 : present.total - 1);
      return;
    }
    // Space belongs to a focused control, so it activates that button rather
    // than stepping the deck past it.
    if (event.key === " " && event.target !== event.currentTarget) return;
    const by = STEPS[event.key];
    if (!by) return;
    event.preventDefault();
    present.step(by);
  }
</script>

<div
  class="present"
  role="presentation"
  tabindex="-1"
  bind:this={layer}
  onkeydown={key}
  onpointerdown={down}
  onpointerup={up}>
  <div class="present-stage" bind:this={stage}>
    <div class="present-card" style="--present-scale: {scale}">
      <div class="present-slide">
        <div class="marpit">{@html card?.html ?? ""}</div>
      </div>
    </div>
  </div>
  <div class="present-hud">
    <Button
      variant="ghost"
      size="sm"
      aria-label="Previous slide"
      disabled={first}
      onclick={() => move(-1)}>←</Button>
    <span class="present-counter">{present.current + 1} / {present.total}</span>
    <Button
      variant="ghost"
      size="sm"
      aria-label="Next slide"
      disabled={last}
      onclick={() => move(1)}>→</Button>
    <Button variant="ghost" size="sm" aria-pressed={notes} onclick={toggleNotes}>Notes</Button>
    <Button variant="ghost" size="sm" onclick={() => present.leave()}>Exit</Button>
  </div>
</div>

<style>
  /* Above the app's own ceiling of 5, so the frontend's title bar is covered
     too. The padding is the lightbox's inset. */
  .present {
    position: fixed;
    inset: 0;
    z-index: 10;
    display: flex;
    padding: var(--space-12);
    background: var(--scrim);
    outline: none;
  }

  /* Nothing here catches a press: the room around a letterboxed slide is scrim,
     and the presenter expects the scrim to let them out. The card's outline and
     shadow fall outside its box, into the inset, so nothing here clips. */
  .present-stage {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: none;
  }

  /* The edge and the shadow belong to the card, at screen scale, rather than to
     the slide inside the transform, where both would thin out. Outline, not
     border, so the box stays the size the slide was scaled to. */
  .present-card {
    flex: none;
    width: calc(1280px * var(--present-scale, 1));
    height: calc(720px * var(--present-scale, 1));
    overflow: hidden;
    background: var(--surface-app);
    outline: var(--border-width) solid var(--border-subtle);
    box-shadow: var(--shadow-menu);
    pointer-events: auto;
  }

  /* Marp's pixel box, scaled from its own corner to fill the card exactly. */
  .present-slide {
    width: 1280px;
    height: 720px;
    transform: scale(var(--present-scale, 1));
    transform-origin: top left;
  }

  .present-hud {
    position: absolute;
    right: 18px;
    bottom: 14px;
    display: flex;
    align-items: center;
    gap: 12px;
    opacity: 0.5;
  }

  .present-hud:hover {
    opacity: 1;
  }

  .present-counter {
    font: var(--machine-sm);
    color: var(--text-muted);
  }
</style>
