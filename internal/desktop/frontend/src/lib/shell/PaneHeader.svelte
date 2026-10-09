<script>
  /** @type {{lead?: import('svelte').Snippet, children?: import('svelte').Snippet, actions?: import('svelte').Snippet, [attribute: string]: any}} */
  let { lead, children, actions, ...rest } = $props();
</script>

<div class="header" {...rest}>
  {#if lead}<span class="lead">{@render lead()}</span>{/if}
  <span class="title">{@render children?.()}</span>
  {#if actions}<span class="actions">{@render actions()}</span>{/if}
</div>

<style>
  .header {
    height: var(--h-pane-header);
    flex: none;
    display: flex;
    align-items: center;
    gap: calc(14px * var(--ui-scale));
    padding: 0 calc(16px * var(--ui-scale));
    border-bottom: 1px solid var(--border-subtle);
    background: var(--surface-chrome);
    position: relative;
    z-index: 4;
  }

  .lead,
  .actions {
    flex: none;
    display: flex;
    gap: calc(8px * var(--ui-scale));
    align-items: center;
  }

  .title {
    flex: 1;
    min-width: 0;
    text-align: center;
  }
</style>
