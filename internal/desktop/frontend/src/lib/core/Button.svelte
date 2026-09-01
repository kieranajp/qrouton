<script>
  /** @type {{variant?: string, size?: string, disabled?: boolean, glyph?: string, wide?: boolean, children?: import('svelte').Snippet, [attribute: string]: any}} */
  let {
    variant = "primary",
    size = "md",
    disabled = false,
    glyph,
    wide = false,
    children,
    ...rest
  } = $props();
</script>

<button class="button {variant} {size}" class:wide {disabled} {...rest}>
  {#if glyph}<span class="glyph" aria-hidden="true">{glyph}</span>{/if}
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

  /* Three pixels short of its column, so the offset shadow has somewhere to fall. */
  .wide {
    width: calc(100% - 3px);
    flex: none;
    padding: 7px 11px;
    gap: 9px;
    justify-content: flex-start;
  }

  .md {
    padding: 9px 18px;
  }

  .primary {
    background: var(--accent-action);
    color: var(--text-on-accent);
    box-shadow: var(--shadow-cube);
  }

  .destructive {
    background: var(--action-destructive);
    color: var(--text-on-accent);
    box-shadow: var(--shadow-cube);
  }

  .secondary,
  .ghost {
    background: transparent;
    border: 1px solid var(--border-default);
    font-weight: 500;
  }

  .secondary {
    color: var(--text-secondary);
  }

  .ghost {
    color: var(--text-muted);
  }

  /* Anything that makes a new thing is a raised object. It is the only shadow in
     the chrome, and the press below is what makes it a cube rather than a
     drawing of one. */
  .cube {
    background: var(--surface-raised);
    border: 1px solid var(--border-default);
    box-shadow: var(--shadow-cube);
    color: var(--text-primary);
  }

  .cube .glyph {
    color: var(--accent-label);
    font-weight: 700;
  }

  .outline {
    background: transparent;
    color: var(--text-primary);
    border: 1px solid var(--accent-action);
  }

  .button:hover:not(:disabled) {
    border-color: var(--ctp-overlay-0);
    color: var(--text-primary);
  }

  .primary:hover:not(:disabled),
  .destructive:hover:not(:disabled) {
    color: var(--text-on-accent);
  }

  .button:active:not(:disabled) {
    transform: translate(2px, 2px);
  }

  .primary:active:not(:disabled),
  .destructive:active:not(:disabled) {
    box-shadow: 1px 1px 0 var(--ctp-surface-0);
  }

  /* The whole offset goes and the button travels the distance it was casting, so
     it sinks flush. A cube that never moves is a lie. */
  .cube:active:not(:disabled) {
    box-shadow: none;
    transform: translate(3px, 3px);
  }

  .button:disabled {
    opacity: 0.45;
  }
</style>
