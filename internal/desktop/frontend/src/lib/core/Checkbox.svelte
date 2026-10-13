<script>
  /** @type {{checked?: boolean, disabled?: boolean, children?: import('svelte').Snippet, [attribute: string]: any}} */
  let { checked = $bindable(false), disabled = false, children, ...rest } = $props();
</script>

<label class="check" class:disabled>
  <input type="checkbox" bind:checked {disabled} {...rest} />
  {#if children}<span class="text">{@render children()}</span>{/if}
</label>

<style>
  .check {
    display: inline-flex;
    align-items: baseline;
    gap: calc(10px * var(--ui-scale));
    min-width: 0;
    cursor: pointer;
  }

  .check.disabled {
    cursor: default;
  }

  input {
    flex: none;
    appearance: none;
    width: calc(14px * var(--ui-scale));
    height: calc(14px * var(--ui-scale));
    margin: 0;
    align-self: center;
    border: var(--border-width, 1px) solid var(--border-default);
    background: var(--surface-chrome);
    cursor: inherit;
  }

  input:hover:not(:disabled) {
    background: var(--wash-selected);
  }

  input:checked {
    border-color: var(--accent-action);
    background: var(--accent-action);
    box-shadow: inset 0 0 0 calc(3px * var(--ui-scale)) var(--surface-chrome);
  }

  input:disabled {
    opacity: 0.5;
  }

  input:focus-visible {
    outline: 1px solid var(--accent-action);
    outline-offset: calc(2px * var(--ui-scale));
  }

  .text {
    min-width: 0;
  }
</style>
