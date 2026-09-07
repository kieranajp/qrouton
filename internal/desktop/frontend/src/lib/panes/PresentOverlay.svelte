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

  // The layer takes the keyboard on mount and hands it back on the way out.
  onMount(() => {
    const beneath = /** @type {HTMLElement | null} */ (document.activeElement);
    layer?.focus();
    const off = Events.On(PRESENTER_CLOSED_EVENT, () => (notes = false));
    return () => {
      off?.();
      beneath?.focus?.();
    };
  });

  // Both dimensions enter the sum: a slide fitted to width alone runs off the
  // bottom of a tall window.
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
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    if (event.key === "Escape") {
      event.preventDefault();
      // Stopped here, so the generic dismisser does not also act on it.
      event.stopPropagation();
      present.leave();
      return;
    }
    if (event.key === "Home" || event.key === "End") {
      event.preventDefault();
      present.go(event.key === "Home" ? 0 : present.total - 1);
      return;
    }
    const by = STEPS[event.key];
    if (!by) return;
    event.preventDefault();
    present.step(by);
  }
</script>

<div class="present" role="presentation" tabindex="-1" bind:this={layer} onkeydown={key}>
  <div class="present-stage" bind:this={stage}>
    <div class="present-slide" style="--present-scale: {scale}">
      <div class="marpit">{@html card?.html ?? ""}</div>
    </div>
  </div>
  <div class="present-hud">
    <span class="present-counter">{present.current + 1} / {present.total}</span>
    <Button variant="ghost" size="sm" aria-pressed={notes} onclick={toggleNotes}>Notes</Button>
    <Button variant="ghost" size="sm" onclick={() => present.leave()}>Exit</Button>
  </div>
</div>

<style>
  /* Above the app's own ceiling of 5, so the frontend's title bar is covered too. */
  .present {
    position: fixed;
    inset: 0;
    z-index: 10;
    background: var(--surface-terminal);
    outline: none;
  }

  .present-stage {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
  }

  /* Marp's pixel box, centred and scaled about its own middle, so the letterbox
     splits evenly whichever dimension is the tight one. */
  .present-slide {
    flex: none;
    width: 1280px;
    height: 720px;
    transform: scale(var(--present-scale, 1));
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
