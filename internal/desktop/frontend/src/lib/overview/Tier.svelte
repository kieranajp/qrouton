<script>
  /** @type {{label: string, name?: string, footer?: boolean, children?: import("svelte").Snippet}} */
  let { label, name = "", footer = false, children } = $props();
</script>

<section class="tier {name}" class:footer aria-label={label}>
  <span class="label">{label}</span>
  <div class="body">{@render children?.()}</div>
</section>

<style>
  .tier {
    display: flex;
    align-items: baseline;
    gap: calc(10px * var(--ui-scale));
    min-width: 0;
    padding-bottom: calc(12px * var(--ui-scale));
  }

  .tier:not(:first-child),
  .footer {
    border-top: 1px solid var(--border-subtle);
    padding-top: calc(12px * var(--ui-scale));
  }

  .footer {
    margin-top: auto;
    padding-bottom: 0;
  }

  .label {
    flex: none;
    width: calc(108px * var(--ui-scale));
    font: var(--instruction);
    letter-spacing: var(--instruction-tracking);
    text-transform: uppercase;
    white-space: nowrap;
    color: var(--text-faint);
  }

  .body {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .footer .body {
    gap: calc(10px * var(--ui-scale));
  }

  @container overview (max-width: 27.5rem) {
    .tier {
      flex-direction: column;
      align-items: stretch;
      gap: calc(6px * var(--ui-scale));
    }

    .label {
      width: auto;
    }
  }
</style>
