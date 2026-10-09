<script>
  /** @type {{title?: string, description?: string, meta?: string, selected?: boolean, layout?: 'row'|'stack', elevated?: boolean, accent?: string, wash?: string, trailing?: import('svelte').Snippet, [attribute: string]: any}} */
  let {
    title,
    description,
    meta,
    selected = false,
    layout = "row",
    elevated = false,
    accent = "var(--accent-action)",
    wash,
    trailing,
    ...rest
  } = $props();
</script>

<div
  class="card {layout}"
  class:selected
  class:elevated
  style:--accent={accent}
  style:--wash={wash || "transparent"}
  {...rest}>
  {#if layout === "stack"}
    <div class="heading">
      <span class="marker"></span>
      <div class="title">{title}</div>
    </div>
    {#if description}<div class="description">{description}</div>{/if}
  {:else}
    <span class="marker"></span>
    <div class="body">
      <div class="title">{title}</div>
      {#if description}<div class="description">{description}</div>{/if}
    </div>
  {/if}
  {#if meta}<span class="meta">{meta}</span>{/if}
  {@render trailing?.()}
</div>

<style>
  .card {
    flex: 1;
    display: flex;
    border: 1px solid var(--border-default);
    background: transparent;
    padding: calc(14px * var(--ui-scale)) calc(16px * var(--ui-scale));
    cursor: pointer;
  }

  .row {
    align-items: center;
    gap: calc(16px * var(--ui-scale));
  }

  .stack {
    flex-direction: column;
    gap: calc(8px * var(--ui-scale));
  }

  .selected {
    border-color: var(--accent);
    background: var(--wash);
  }

  .selected.elevated {
    box-shadow: var(--shadow-focus-md);
  }

  .heading {
    display: flex;
    align-items: center;
    gap: calc(9px * var(--ui-scale));
  }

  .marker {
    width: calc(12px * var(--ui-scale));
    height: calc(12px * var(--ui-scale));
    flex: none;
    box-sizing: border-box;
    background: transparent;
    border: 1px solid var(--border-default);
  }

  .selected .marker {
    background: var(--accent);
    border: none;
  }

  .body {
    flex: 1;
  }

  .title {
    font: var(--display-xs);
    letter-spacing: var(--display-tracking);
    color: var(--text-secondary);
  }

  .selected .title {
    color: var(--text-primary);
  }

  .description {
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .row .description {
    margin-top: calc(4px * var(--ui-scale));
  }

  .meta {
    font: var(--literal);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-faint);
  }
</style>
