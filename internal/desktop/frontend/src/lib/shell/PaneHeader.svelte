<script>
  /** @type {{children?: import('svelte').Snippet, actions?: import('svelte').Snippet, [attribute: string]: any}} */
  let { children, actions, ...rest } = $props();
</script>

<div class="header" {...rest}>
  {@render children?.()}
  {#if actions}<span class="actions">{@render actions()}</span>{/if}
</div>

<style>
  .header {
    height: var(--h-pane-header);
    flex: none;
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 0 16px;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--surface-chrome);
    position: relative;
    z-index: 4;
  }

  .actions {
    margin-left: auto;
    display: flex;
    gap: 8px;
    align-items: center;
  }
</style>
