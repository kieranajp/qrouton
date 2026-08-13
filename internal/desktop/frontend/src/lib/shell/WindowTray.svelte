<script>
  /** @type {{summary?: string, [attribute: string]: any}} */
  let { summary, ...rest } = $props();
</script>

<div class="footer" {...rest}>
  {#if summary}<span>{summary}</span>{/if}
</div>

<style>
  .footer {
    height: var(--h-status-footer);
    flex: none;
    background: var(--surface-chrome);
    border-top: 1px solid var(--border-subtle);
    display: flex;
    align-items: center;
    justify-content: flex-end;
    padding: 0 16px;
  }

  span {
    font: var(--machine-md);
    font-size: 11px;
    color: var(--text-muted);
  }
</style>
