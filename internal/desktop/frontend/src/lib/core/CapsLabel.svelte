<script>
  /** @type {{tone?: 'active'|'dim', as?: string, children?: import('svelte').Snippet, [attribute: string]: any}} */
  let { tone = "active", as = "span", children, ...rest } = $props();
</script>

<svelte:element this={as} class="caps {tone}" {...rest}>{@render children?.()}</svelte:element>

<style>
  .caps {
    text-transform: uppercase;
  }

  .active {
    font: var(--instruction);
    letter-spacing: var(--instruction-tracking);
    color: var(--accent-label);
  }

  .dim {
    font: var(--instruction-sm);
    letter-spacing: var(--instruction-tracking-sm);
    color: var(--text-faint);
  }
</style>
