<script>
  import { onMount } from "svelte";
  import Button from "../core/Button.svelte";
  import ImageMenu from "./ImageMenu.svelte";

  /** @type {{image: {source: string, path?: string, url: string}, aspect: number, index?: number, total?: number, slug?: string, onStep?: (by: number) => void, onClose: () => void}} */
  let { image, aspect, index = 1, total = 1, slug = "", onStep, onClose } = $props();

  /** @type {{image: {source: string, path?: string}, x: number, y: number} | null} */
  let menu = $state(null);
  let first = $derived(index <= 1);
  let last = $derived(index >= total);
  // The pane's own notice is behind the scrim, so what the menu did is said here.
  let notice = $state("");
  let noticeTimer;

  function say(text) {
    notice = text;
    clearTimeout(noticeTimer);
    noticeTimer = setTimeout(() => (notice = ""), 1600);
  }

  let layer = $state(/** @type {HTMLElement | undefined} */ (undefined));
  let stage = $state(/** @type {HTMLElement | undefined} */ (undefined));
  let hud = $state(/** @type {HTMLElement | undefined} */ (undefined));
  let width = $state(0);
  let height = $state(0);
  let from = false;

  onMount(() => {
    layer?.focus();
    return () => clearTimeout(noticeTimer);
  });

  $effect(() => {
    if (!stage) return;
    const ratio = aspect;
    const observer = new ResizeObserver(([entry]) => {
      const room = entry.contentRect;
      if (room.width / room.height > ratio) {
        height = room.height;
        width = height * ratio;
      } else {
        width = room.width;
        height = width / ratio;
      }
    });
    observer.observe(stage);
    return () => observer.disconnect();
  });

  function down(event) {
    from = event.target === event.currentTarget;
  }

  function up(event) {
    const scrim = from && event.target === event.currentTarget;
    from = false;
    if (scrim) onClose();
  }

  function step(by) {
    if (by < 0 ? !first : !last) onStep?.(by);
  }

  function openMenu(event) {
    // The webview draws one of its own otherwise.
    event.preventDefault();
    menu = { image, x: event.clientX, y: event.clientY };
  }

  function key(event) {
    event.stopPropagation();
    if (event.key === "Escape") {
      event.preventDefault();
      onClose();
    } else if (event.key === "Tab") {
      event.preventDefault();
      hud?.querySelector("button")?.focus();
    } else if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
      event.preventDefault();
      step(event.key === "ArrowLeft" ? -1 : 1);
    }
  }
</script>

<div
  class="lightbox"
  role="dialog"
  aria-modal="true"
  aria-label={`Expanded image: ${image.source}`}
  tabindex="-1"
  bind:this={layer}
  onkeydown={key}
  onpointerdown={down}
  onpointerup={up}>
  <div class="stage" bind:this={stage}>
    <div
      class="card"
      style:width={`${width}px`}
      style:height={`${height}px`}
      oncontextmenu={openMenu}
      role="presentation">
      <img src={image.url} alt={image.source} />
    </div>
  </div>
  <div class="hud" bind:this={hud}>
    {#if notice}<span class="counter" role="status">{notice}</span>{/if}
    <Button
      variant="ghost"
      size="sm"
      aria-label="Previous image"
      disabled={first}
      onclick={() => step(-1)}>←</Button>
    <span class="counter">{index} / {total}</span>
    <Button
      variant="ghost"
      size="sm"
      aria-label="Next image"
      disabled={last}
      onclick={() => step(1)}>→</Button>
    <Button variant="ghost" size="sm" onclick={onClose}>Close</Button>
  </div>
  {#if menu}
    <ImageMenu {slug} at={menu} onDismiss={() => (menu = null)} onNotice={say} />
  {/if}
</div>

<style>
  .lightbox {
    position: fixed;
    inset: 0;
    z-index: 10;
    display: flex;
    padding: var(--space-12);
    background: var(--scrim);
    outline: none;
  }

  .stage {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: none;
  }

  .card {
    flex: none;
    overflow: hidden;
    background: var(--surface-app);
    outline: var(--border-width) solid var(--border-subtle);
    box-shadow: var(--shadow-menu);
    pointer-events: auto;
  }

  img {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: contain;
  }

  .hud {
    position: absolute;
    right: 18px;
    bottom: 14px;
    display: flex;
    align-items: center;
    gap: 6px;
    opacity: 0.5;
  }

  .counter {
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .hud:hover,
  .hud:focus-within {
    opacity: 1;
  }
</style>
