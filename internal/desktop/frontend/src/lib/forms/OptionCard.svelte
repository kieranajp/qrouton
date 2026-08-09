<script>
  /** @type {{title?: string, description?: string, meta?: string, selected?: boolean, accent?: string, wash?: string, [attribute: string]: any}} */
  let {
    title,
    description,
    meta,
    selected = false,
    accent = "var(--accent-action)",
    wash,
    ...rest
  } = $props();
</script>

<div
  class="card"
  class:selected
  style:--accent={accent}
  style:--wash={wash || "transparent"}
  {...rest}>
  <span class="marker"></span>
  <div class="body">
    <div class="title">{title}</div>
    {#if description}<div class="description">{description}</div>{/if}
  </div>
  {#if meta}<span class="meta">{meta}</span>{/if}
</div>

<style>
  .card {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 16px;
    border: 1px solid var(--border-default);
    background: transparent;
    padding: 14px 16px;
    cursor: pointer;
  }

  .selected {
    border-color: var(--accent);
    background: var(--wash);
    box-shadow: var(--shadow-focus);
  }

  .marker {
    width: 12px;
    height: 12px;
    flex: none;
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
    margin-top: 4px;
  }

  .meta {
    font: var(--terminal-sm);
    font-size: 11px;
    color: var(--text-faint);
  }
</style>
