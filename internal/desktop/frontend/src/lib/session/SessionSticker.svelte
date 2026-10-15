<script>
  import StickerIcon from "./StickerIcon.svelte";
  import { sticker, stickerCss } from "./stickers.js";
  let { id = "", stickers = [] } = $props();
  let item = $derived(sticker(id, stickers));
</script>
<span class="sticker-display" style:color={item ? stickerCss(item.colour) : undefined} aria-hidden="true">
  {#if item}<StickerIcon symbol={item.symbol} />{:else}<StickerIcon outline />{/if}
</span>
<style>.sticker-display { display:inline-flex; font:400 calc(15px * var(--ui-scale))/1 var(--font-machine); color:var(--text-faint); }</style>
