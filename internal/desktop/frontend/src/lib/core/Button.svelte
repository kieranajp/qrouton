<script>
  /** @type {{variant?: string, size?: string, disabled?: boolean, glyph?: string, children?: import('svelte').Snippet, [attribute: string]: any}} */
  let { variant = "primary", size = "md", disabled = false, glyph, children, ...rest } = $props();
</script>

<button class="button {variant} {size}" {disabled} {...rest}>
  {#if glyph}<span aria-hidden="true">{glyph}</span>{/if}
  {@render children?.()}
</button>

<style>
  .button {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    border: 0;
    border-radius: 0;
    cursor: pointer;
    font: var(--button);
    white-space: nowrap;
  }

  .sm {
    padding: 6px 11px;
    font-size: 11px;
  }

  .md {
    padding: 9px 18px;
  }

  .primary {
    background: var(--accent-action);
    color: var(--text-on-accent);
    box-shadow: var(--shadow-cube);
  }

  .secondary,
  .ghost,
  .dashed {
    background: transparent;
    border: 1px solid var(--border-default);
    font-weight: 500;
  }

  .secondary {
    color: var(--text-secondary);
  }

  .ghost,
  .dashed {
    color: var(--text-muted);
  }

  .dashed {
    border-style: dashed;
    justify-content: flex-start;
    width: 100%;
  }

  .outline {
    background: transparent;
    color: var(--text-primary);
    border: 1px solid var(--accent-action);
  }

  /* Press collapses the offset shadow, so the cube reads as pushed in. */
  .button:hover:not(:disabled) {
    border-color: var(--ctp-overlay-0);
    color: var(--text-primary);
  }

  .primary:hover:not(:disabled) {
    color: var(--text-on-accent);
  }

  .button:active:not(:disabled) {
    transform: translate(2px, 2px);
  }

  .primary:active:not(:disabled) {
    box-shadow: 1px 1px 0 var(--ctp-surface-0);
  }

  .button:disabled {
    opacity: 0.45;
  }
</style>
