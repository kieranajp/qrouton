<script>
  import { onDestroy, tick } from "svelte";
  import { WINDOWS_FOCUS_IMAGE } from "../bridge/generated.js";
  import { Call } from "../wails.js";

  /** @type {{doc: {images?: {source: string, url: string}[], currentIndex?: number, revision?: number}, id: string, slug?: string, active?: boolean, scrollRoot?: HTMLElement}} */
  let { doc, id, slug = "", active = false, scrollRoot } = $props();

  let images = $derived(doc.images ?? []);
  let current = $derived(doc.currentIndex ?? 1);
  let entry = $derived(images[current - 1]);
  /** @type {Record<string, 'loaded' | 'error'>} */
  let loads = $state({});
  let selectionError = $state("");
  /** @type {HTMLOListElement} */
  let strip = $state();
  let alive = true;
  let request = 0;
  onDestroy(() => { alive = false; });

  async function select(index) {
    const attempt = ++request;
    const revision = doc.revision;
    selectionError = "";
    try {
      await Call.ByName(WINDOWS_FOCUS_IMAGE, slug, id, index);
    } catch {
      if (alive && attempt === request && doc.revision === revision) {
        selectionError = "Could not select image. Try again.";
      }
    }
  }

  $effect(() => {
    const revision = doc.revision;
    const index = current;
    selectionError = "";
    if (!active) return;
    let cancelled = false;
    tick().then(() => {
      if (cancelled || !alive || revision !== doc.revision) return;
      if (scrollRoot) scrollRoot.scrollTop = 0;
      const item = strip?.children[index - 1];
      if (item) {
        const box = item.getBoundingClientRect();
        const port = strip.getBoundingClientRect();
        if (box.left < port.left) strip.scrollLeft -= port.left - box.left;
        else if (box.right > port.right) strip.scrollLeft += box.right - port.right;
      }
    });
    return () => { cancelled = true; };
  });

  const basename = (source) => source.split("/").pop();
</script>

<article class="images-pane" aria-label="Image gallery">
  {#if entry}
    <figure class="primary">
      <figcaption>
        <span class="position">Image {current} of {images.length}</span>
        <strong>{basename(entry.source)}</strong>
        <span class="path">{entry.source}</span>
      </figcaption>
      <div class="primary-frame">
        {#if loads[entry.url] === "error"}
          <p class="load-error">Could not load image<br />{entry.source}</p>
        {:else}
          {#if !loads[entry.url]}<span class="loading">Loading image…</span>{/if}
          <img src={entry.url} alt={entry.source}
            onload={() => (loads[entry.url] = "loaded")}
            onerror={() => (loads[entry.url] = "error")} />
        {/if}
      </div>
    </figure>
  {/if}
  {#if selectionError}<p class="selection-error" role="alert">{selectionError}</p>{/if}
  <ol bind:this={strip} class="image-strip" aria-label="Images in supplied order">
    {#each images as image, index (image.url)}
      <li>
        <button type="button" class:current={index + 1 === current}
          aria-pressed={index + 1 === current} aria-label={`Image ${index + 1}: ${image.source}`}
          onclick={() => select(index + 1)}>
        <span class="number">{index + 1}</span>
        <div class="thumbnail-frame">
          {#if loads[image.url] === "error"}
            <span class="load-error">Could not load image</span>
          {:else}
            {#if !loads[image.url]}<span class="loading">Loading…</span>{/if}
            <img src={image.url} alt={image.source}
              onload={() => (loads[image.url] = "loaded")}
              onerror={() => (loads[image.url] = "error")} />
          {/if}
        </div>
        <strong>{basename(image.source)}</strong>
        <span class="path">{image.source}</span>
        </button>
      </li>
    {/each}
  </ol>
</article>

<style>
  .images-pane { padding: 16px; min-width: 0; font: var(--machine-sm); }
  .primary { margin: 0 0 18px; min-width: 0; }
  figcaption { display: flex; flex-direction: column; gap: 4px; margin-bottom: 12px; }
  .position, .number { color: var(--accent-action); font-weight: 700; }
  strong, .path { overflow-wrap: anywhere; }
  .path { display: block; color: var(--text-muted); font: var(--machine-xs); }
  .primary-frame, .thumbnail-frame { display: flex; justify-content: center; align-items: center; position: relative; background: var(--surface-raised); border-radius: 4px; }
  .primary-frame { min-height: 180px; padding: 12px; }
  img { display: block; width: auto; height: auto; max-width: 100%; object-fit: contain; }
  .primary-frame img { max-height: min(55vh, 560px); }
  .image-strip { display: flex; gap: 10px; overflow-x: auto; margin: 0; padding: 4px 2px 12px; list-style: none; }
  li { flex: 0 0 150px; min-width: 0; }
  button { width: 100%; height: 100%; text-align: left; cursor: pointer; background: transparent; color: inherit; font: inherit; box-sizing: border-box; padding: 8px; border: 2px solid var(--border-subtle); border-radius: 6px; }
  button.current { border-color: var(--accent-action); }
  button:focus-visible { outline: 2px solid var(--accent-action); outline-offset: 2px; }
  .selection-error { color: var(--text-secondary); }
  .thumbnail-frame { height: 90px; margin: 6px 0; }
  .thumbnail-frame img { max-height: 90px; }
  .loading { position: absolute; color: var(--text-muted); }
  .load-error { color: var(--text-secondary); overflow-wrap: anywhere; text-align: center; }
</style>
