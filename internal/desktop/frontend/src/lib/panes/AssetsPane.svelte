<script>
  import { onDestroy, tick } from "svelte";
  import { GALLERY_LABEL, MEDIA_KINDS } from "../bridge/generated.js";
  import Chip from "../core/Chip.svelte";
  import { linux } from "../platform.js";
  import { focusAsset } from "./calls.js";
  import ImageLightbox from "./ImageLightbox.svelte";
  import AssetMenu from "./AssetMenu.svelte";
  import ArtifactHeader from "./ArtifactHeader.svelte";

  /** @type {{doc: {assets?: {source: string, path?: string, url: string, kind: import("../bridge/generated.js").MediaKind}[], currentIndex?: number, revision?: number}, id: string, slug?: string, active?: boolean, scrollRoot?: HTMLElement}} */
  let { doc, id, slug = "", active = false, scrollRoot } = $props();

  let assets = $derived(doc.assets ?? []);
  let current = $derived(doc.currentIndex ?? 1);
  let entry = $derived(assets[current - 1]);
  /** @type {Record<string, 'loaded' | 'error' | 'unsupported'>} */
  let loads = $state({});
  /** @type {Record<string, 'loaded' | 'error'>} */
  let thumbs = $state({});
  /** @type {Record<string, number>} */
  let aspects = $state({});
  let selectionError = $state("");
  let expanded = $state(false);
  /** @type {{asset: {source: string, path?: string, kind: import("../bridge/generated.js").MediaKind}, x: number, y: number} | null} */
  let menu = $state(null);
  let notice = $state("");
  let noticeTimer;
  /** @type {HTMLButtonElement} */
  let primary = $state();
  /** @type {HTMLVideoElement | undefined} */
  let player = $state();
  /** @type {HTMLOListElement} */
  let strip = $state();
  let alive = true;
  let request = 0;
  onDestroy(() => { alive = false; clearTimeout(noticeTimer); });

  function openMenu(event, asset) {
    // The webview draws one of its own otherwise, and the workbench's own
    // text menu stands down on the same signal.
    event.preventDefault();
    menu = { asset, x: event.clientX, y: event.clientY };
  }

  function show(text) {
    notice = text;
    clearTimeout(noticeTimer);
    noticeTimer = setTimeout(() => (notice = ""), 1600);
  }

  function stepImages(by) {
    for (let next = current + by; next >= 1 && next <= assets.length; next += by) {
      if (!isVideo(assets[next - 1])) return select(next);
    }
  }

  function loaded(states, url, event) {
    states[url] = "loaded";
    const image = /** @type {HTMLImageElement} */ (event.currentTarget);
    aspects[url] = image.naturalWidth / image.naturalHeight;
  }

  function unplayable(url, event) {
    const video = /** @type {HTMLVideoElement} */ (event.currentTarget);
    loads[url] = video.error?.code === MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED ? "unsupported" : "error";
  }

  const reseeked = new WeakSet();
  function unpainted(event) {
    const video = /** @type {HTMLVideoElement} */ (event.currentTarget);
    if (video.readyState !== HTMLMediaElement.HAVE_METADATA || reseeked.has(video)) return;
    reseeked.add(video);
    video.currentTime = 0.1;
  }

  async function closeLightbox() {
    expanded = false;
    await tick();
    if (active) (primary ?? strip?.children[current - 1]?.querySelector("button"))?.focus();
  }

  async function select(index) {
    const attempt = ++request;
    const revision = doc.revision;
    selectionError = "";
    const answer = await focusAsset(slug, id, index);
    if (!answer.ok && alive && attempt === request && doc.revision === revision) {
      selectionError = "Could not select asset. Try again.";
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

  $effect(() => {
    if (expanded && entry && isVideo(entry)) closeLightbox();
  });

  $effect(() => {
    if (!active) {
      expanded = false;
      menu = null;
      player?.pause();
    }
  });

  const basename = (source) => source.split("/").pop();
  const isVideo = (asset) => asset.kind === MEDIA_KINDS.VIDEO;
  const noun = (asset) => (isVideo(asset) ? "Video" : "Image");
</script>

<article class="assets-pane" aria-label="Asset gallery">
  {#if entry}
    <ArtifactHeader source={entry.source} path={entry.path} face="var(--accent-label)">
      {#snippet tag()}<Chip>{GALLERY_LABEL}</Chip>{/snippet}
    </ArtifactHeader>
    <figure class="primary">
      <figcaption>
        <span class="position">{noun(entry)} {current} of {assets.length}</span>
      </figcaption>
      {#key entry.url}
      {@const shown = entry}
      {#if isVideo(shown)}
      <div class="primary-frame player" role="presentation" oncontextmenu={(event) => openMenu(event, shown)}>
        {#if loads[shown.url] === "unsupported"}
          <p class="load-error">This video's format can't be played here{#if linux}<br />Install gstreamer1.0-plugins-good and gstreamer1.0-libav.{/if}</p>
        {:else if loads[shown.url] === "error"}
          <p class="load-error">Could not play video</p>
        {:else}
          {#if !loads[shown.url]}<span class="loading">Loading video…</span>{/if}
          <!-- svelte-ignore a11y_media_has_caption -->
          <video bind:this={player} src={shown.url} controls preload="metadata" aria-label={shown.source}
            onloadedmetadata={() => (loads[shown.url] = "loaded")}
            onerror={(event) => unplayable(shown.url, event)}></video>
        {/if}
      </div>
      {:else}
      <button
        type="button"
        class="primary-frame"
        bind:this={primary}
        disabled={loads[shown.url] !== "loaded"}
        aria-label={`Expand image: ${shown.source}`}
        oncontextmenu={(event) => openMenu(event, shown)}
        onclick={() => (expanded = true)}>
        {#if loads[shown.url] === "error"}
          <p class="load-error">Could not load image<br />{shown.source}</p>
        {:else}
          {#if !loads[shown.url]}<span class="loading">Loading image…</span>{/if}
          <img src={shown.url} alt={shown.source}
            onload={(event) => loaded(loads, shown.url, event)}
            onerror={() => (loads[shown.url] = "error")} />
        {/if}
      </button>
      {/if}
      {/key}
    </figure>
  {/if}
  {#if selectionError}<p class="selection-error" role="alert">{selectionError}</p>{/if}
  {#if notice}<p class="notice" role="status">{notice}</p>{/if}
  <ol bind:this={strip} class="asset-strip" aria-label="Assets in supplied order">
    {#each assets as asset, index (asset.url)}
      <li>
        <button type="button" class:current={index + 1 === current}
          aria-pressed={index + 1 === current} aria-label={`${noun(asset)} ${index + 1}: ${asset.source}`}
          oncontextmenu={(event) => openMenu(event, asset)}
          onclick={() => select(index + 1)}>
        <span class="number">{index + 1}</span>
        <div class="thumbnail-frame">
          {#if isVideo(asset)}
            {#if thumbs[asset.url] !== "error"}
              {#if !thumbs[asset.url]}<span class="loading">Loading…</span>{/if}
              <video src={asset.url + "#t=0.1"} muted playsinline preload="metadata" aria-hidden="true"
                onloadedmetadata={() => (thumbs[asset.url] = "loaded")}
                onerror={() => (thumbs[asset.url] = "error")}
                onsuspend={unpainted}></video>
            {/if}
            <span class="play" aria-hidden="true"></span>
          {:else if thumbs[asset.url] === "error"}
            <span class="load-error">Could not load image</span>
          {:else}
            {#if !thumbs[asset.url]}<span class="loading">Loading…</span>{/if}
            <img src={asset.url} alt={asset.source}
              onload={(event) => loaded(thumbs, asset.url, event)}
              onerror={() => (thumbs[asset.url] = "error")} />
          {/if}
        </div>
        <strong>{basename(asset.source)}</strong>
        <span class="path">{asset.source}</span>
        </button>
      </li>
    {/each}
  </ol>
</article>

{#if expanded && active && entry && !isVideo(entry)}
  <ImageLightbox
    image={entry}
    aspect={aspects[entry.url] ?? 1}
    index={current}
    total={assets.length}
    {slug}
    onStep={stepImages}
    onClose={closeLightbox} />
{/if}

{#if menu}
  <AssetMenu {slug} at={menu} onDismiss={() => (menu = null)} onNotice={show} />
{/if}

<style>
  .assets-pane {
    min-width: 0;
    font: var(--machine-sm);
  }

  .primary {
    margin: 0 calc(16px * var(--ui-scale)) calc(18px * var(--ui-scale));
    min-width: 0;
  }

  figcaption {
    display: flex;
    gap: calc(4px * var(--ui-scale));
    margin-bottom: calc(12px * var(--ui-scale));
  }

  .position, .number {
    color: var(--accent-action);
    font-weight: 700;
  }

  strong, .path {
    overflow-wrap: anywhere;
  }

  .path {
    display: block;
    color: var(--text-muted);
    font: var(--machine-xs);
  }

  .primary-frame, .thumbnail-frame {
    display: flex;
    justify-content: center;
    align-items: center;
    position: relative;
    background: var(--surface-raised);
    border-radius: calc(4px * var(--ui-scale));
  }

  .primary-frame {
    width: 100%;
    min-height: calc(180px * var(--ui-scale));
    padding: calc(12px * var(--ui-scale));
    border: 0;
    color: inherit;
    font: inherit;
    cursor: zoom-in;
  }

  .primary-frame:disabled {
    cursor: default;
  }

  .primary-frame:focus-visible {
    outline: 2px solid var(--accent-action);
    outline-offset: -2px;
  }

  img, video {
    display: block;
    width: auto;
    height: auto;
    max-width: 100%;
    object-fit: contain;
  }

  .primary-frame.player {
    cursor: auto;
  }

  .primary-frame img, .primary-frame video {
    max-height: min(55vh, calc(560px * var(--ui-scale)));
  }

  .asset-strip {
    display: flex;
    gap: calc(10px * var(--ui-scale));
    overflow-x: auto;
    margin: 0;
    padding: calc(4px * var(--ui-scale)) calc(18px * var(--ui-scale)) calc(28px * var(--ui-scale));
    list-style: none;
  }

  li {
    flex: 0 0 calc(150px * var(--ui-scale));
    min-width: 0;
  }

  .asset-strip button {
    width: 100%;
    height: 100%;
    text-align: left;
    cursor: pointer;
    background: transparent;
    color: inherit;
    font: inherit;
    box-sizing: border-box;
    padding: calc(8px * var(--ui-scale));
    border: 2px solid var(--border-subtle);
    border-radius: calc(6px * var(--ui-scale));
  }

  .asset-strip button.current {
    border-color: var(--accent-action);
  }

  .asset-strip button:focus-visible {
    outline: 2px solid var(--accent-action);
    outline-offset: 2px;
  }

  .selection-error {
    color: var(--text-secondary);
  }

  .notice {
    color: var(--text-muted);
  }

  .thumbnail-frame {
    height: calc(90px * var(--ui-scale));
    margin: calc(6px * var(--ui-scale)) 0;
  }

  .thumbnail-frame img, .thumbnail-frame video {
    max-height: calc(90px * var(--ui-scale));
  }

  .play {
    position: absolute;
    width: calc(28px * var(--ui-scale));
    height: calc(28px * var(--ui-scale));
    border-radius: 50%;
    background: rgb(0 0 0 / 55%);
  }

  .play::after {
    content: "";
    position: absolute;
    inset: 28% 26% 28% 36%;
    background: var(--text-primary);
    clip-path: polygon(0 0, 100% 50%, 0 100%);
  }

  .loading {
    position: absolute;
    color: var(--text-muted);
  }

  .load-error {
    color: var(--text-secondary);
    overflow-wrap: anywhere;
    text-align: center;
  }

</style>
