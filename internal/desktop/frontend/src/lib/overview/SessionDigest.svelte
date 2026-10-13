<script>
  import { digestMeta } from "./digest.js";
  import Tier from "./Tier.svelte";

  /** @type {{digest?: import("./digest.js").SessionDigest, now: number}} */
  let { digest, now } = $props();

  let shown = $derived(digestMeta(digest, now));
</script>

{#if shown}
  <Tier label="So far" name="digest">
    <p class="text">{shown.text}</p>
    {#if shown.meta}<p class="meta">{shown.meta}</p>{/if}
  </Tier>
{/if}

<style>
  .text,
  .meta {
    margin: 0;
    font: var(--machine-xs);
  }

  .text {
    white-space: pre-line;
    overflow-wrap: break-word;
    color: var(--text-primary);
  }

  .meta {
    color: var(--text-faint);
  }
</style>
