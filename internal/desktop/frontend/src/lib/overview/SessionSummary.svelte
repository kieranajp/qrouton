<script>
  import { summaryMeta } from "./summary.js";

  /** @type {{summary?: import("./summary.js").SessionSummary, now: number}} */
  let { summary, now } = $props();

  let shown = $derived(summaryMeta(summary, now));
</script>

<div class="summary" class:empty={shown.empty}>
  <p class="text">{shown.text}</p>
  <p class="meta">{shown.meta}</p>
</div>

<style>
  .summary {
    display: flex;
    flex-direction: column;
    gap: calc(6px * var(--ui-scale));
    padding-bottom: calc(14px * var(--ui-scale));
  }

  .text,
  .meta {
    margin: 0;
  }

  .text {
    font: 500 calc(15px * var(--ui-scale)) / 1.4 var(--font-display);
    letter-spacing: -0.01em;
    text-wrap: pretty;
    overflow-wrap: break-word;
    color: var(--text-primary);
  }

  .meta {
    font: var(--machine-xs);
    color: var(--text-faint);
  }

  .empty .text {
    color: var(--text-faint);
  }
</style>
