<script>
  /** @type {{title?: string, children?: import('svelte').Snippet}} */
  let { title, children } = $props();
</script>

<div class="heading">
  <span class="title">{title}</span>
  <span class="helper">{@render children?.()}</span>
</div>

<style>
  .heading {
    display: flex;
    align-items: flex-end;
    gap: calc(18px * var(--ui-scale));
  }

  .title {
    font: var(--display-md);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  .helper {
    flex: 1;
    font: var(--machine-sm);
    color: var(--text-muted);
    padding-bottom: calc(5px * var(--ui-scale));
  }
</style>
