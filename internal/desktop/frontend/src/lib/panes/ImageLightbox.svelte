<script>
  import { onMount } from "svelte";
  import Button from "../core/Button.svelte";

  /** @type {{image: {source: string, url: string}, aspect: number, onClose: () => void}} */
  let { image, aspect, onClose } = $props();

  let layer = $state(/** @type {HTMLElement | undefined} */ (undefined));
  let stage = $state(/** @type {HTMLElement | undefined} */ (undefined));
  let hud = $state(/** @type {HTMLElement | undefined} */ (undefined));
  let width = $state(0);
  let height = $state(0);
  let from = false;

  onMount(() => {
    layer?.focus();
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

  function key(event) {
    event.stopPropagation();
    if (event.key === "Escape") {
      event.preventDefault();
      onClose();
    } else if (event.key === "Tab") {
      event.preventDefault();
      hud?.querySelector("button")?.focus();
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
    <div class="card" style:width={`${width}px`} style:height={`${height}px`}>
      <img src={image.url} alt={image.source} />
    </div>
  </div>
  <div class="hud" bind:this={hud}>
    <Button variant="ghost" size="sm" onclick={onClose}>Close</Button>
  </div>
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
    opacity: 0.5;
  }

  .hud:hover,
  .hud:focus-within {
    opacity: 1;
  }
</style>
