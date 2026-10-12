<script>
  /** @type {{title: string, id?: string, children?: import('svelte').Snippet, aside?: import('svelte').Snippet}} */
  let { title, id, children, aside } = $props();
</script>

<header class="tab-header">
  <div class="words">
    <h2 {id}>{title}</h2>
    {#if children}<p>{@render children()}</p>{/if}
  </div>
  {@render aside?.()}
</header>

<style>
  .tab-header {
    display: flex;
    align-items: flex-start;
    gap: calc(20px * var(--ui-scale));
  }

  .words {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: calc(6px * var(--ui-scale));
  }

  h2 {
    margin: 0;
    font: var(--display-xs);
    letter-spacing: -0.03em;
    color: var(--text-primary);
  }

  p {
    margin: 0;
    max-width: calc(640px * var(--ui-scale));
    font: var(--machine-sm);
    color: var(--text-muted);
    text-wrap: pretty;
  }
</style>
