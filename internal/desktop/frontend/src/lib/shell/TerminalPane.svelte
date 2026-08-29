<script>
  /** @type {{lines?: string[], size?: 'md'|'sm', flush?: boolean, children?: import('svelte').Snippet, [attribute: string]: any}} */
  let { lines = [], size = "md", flush = false, children, ...rest } = $props();
</script>

<div class="pane" class:small={size === "sm"} class:flush {...rest}>
  {#if children}
    {@render children()}
  {:else}
    {#each lines as line, i (i)}
      <div class="line">{line === "" ? " " : line}</div>
    {/each}
  {/if}
</div>

<style>
  .pane {
    flex: 1;
    min-height: 0;
    background: var(--surface-terminal);
    padding: 12px 14px;
    /* flex-basis sizes the content box, so without this the padding overflows. */
    box-sizing: border-box;
    font: var(--terminal);
    color: var(--text-primary);
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .small {
    font: var(--terminal-sm);
  }

  /* For content that reaches the pane's edges and insets itself. */
  .flush {
    padding: 0;
  }

  .line {
    white-space: pre;
  }
</style>
