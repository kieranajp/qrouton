<script>
  import { STICKER_GLYPHS, stickerCss } from "./stickers.js";

  /** @type {{symbol?: string, colour?: string, size?: number, outline?: boolean}} */
  let { symbol = "", colour = "", size = 14, outline = false } = $props();

  let drawn = $derived(outline ? STICKER_GLYPHS.star : STICKER_GLYPHS[symbol]);
</script>

<svg
  class="sticker-icon"
  width={size}
  height={size}
  viewBox="0 0 16 16"
  data-sticker={outline ? "none" : symbol}
  style:color={colour ? stickerCss(colour) : undefined}
  fill="none"
  aria-hidden="true">
  {#if outline}
    <path d={drawn.fill} fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round" />
  {:else if drawn}
    {#if drawn.fill}<path d={drawn.fill} fill="currentColor" />{/if}
    {#if drawn.stroke}
      <path
        d={drawn.stroke}
        fill="none"
        stroke="currentColor"
        stroke-width={drawn.width}
        stroke-linecap="round"
        stroke-linejoin="round" />
    {/if}
  {/if}
</svg>

<style>
  .sticker-icon {
    display: block;
    flex: none;
  }
</style>
