<script>
  import { dismissible } from "../core/dismiss.js";

  /** Counts every session-written document without implying authorship.
   * @type {{count?: number, open?: boolean, unseen?: boolean, onToggle?: () => void, children?: import('svelte').Snippet, [attribute: string]: any}} */
  let { count = 0, open = false, unseen = false, onToggle, children, ...rest } = $props();

  const dismiss = () => open && onToggle?.();
</script>

<div class="index" use:dismissible={dismiss} {...rest}>
  <button
    class="chip"
    class:lit={open || unseen}
    disabled={!onToggle}
    aria-expanded={open}
    onclick={onToggle}>
    <span>Documents</span>
    <span class="count">{count}</span>
    <span class="caret" aria-hidden="true">&#9662;</span>
  </button>
  {#if open}{@render children?.()}{/if}
</div>

<style>
  .index {
    display: flex;
    align-items: center;
    position: relative;
    flex: none;
  }

  .chip {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--surface-raised);
    color: var(--text-primary);
    border: 1px solid var(--border-default);
    border-radius: 0;
    font: var(--machine-sm);
    padding: 4px 8px;
    cursor: pointer;
  }

  .chip:hover:enabled,
  .lit {
    border-color: var(--accent-action);
  }

  .chip:disabled {
    color: var(--text-muted);
    cursor: default;
  }

  .count {
    font: var(--machine-xs);
    font-size: 10px;
    color: var(--text-faint);
  }

  .caret {
    color: var(--text-muted);
  }
</style>
