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
    padding: 14px 16px;
    cursor: pointer;
  }

  .row {
    align-items: center;
    gap: 16px;
  }

  .stack {
    flex-direction: column;
    gap: 8px;
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
    gap: 9px;
  }

  .marker {
    width: 12px;
    height: 12px;
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
    margin-top: 4px;
  }

  .meta {
    font: var(--literal);
    font-size: 11px;
    color: var(--text-faint);
  }
</style>
