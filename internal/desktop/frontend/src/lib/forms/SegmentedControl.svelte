<script>
  /** @type {{segments?: {key: string, label: string, accent?: string, ink?: string, disabled?: boolean}[], value?: string|string[], multiple?: boolean, size?: 'sm'|'md', disabled?: boolean, onSelect?: (key: string) => void, [attribute: string]: any}} */
  let {
    segments = [],
    value,
    multiple = false,
    size = "md",
    disabled = false,
    onSelect,
    ...rest
  } = $props();

  let chosen = $derived(multiple ? (Array.isArray(value) ? value : []) : [value]);
</script>

<div class="control" {...rest}>
  {#each segments as segment (segment.key)}
    <button
      type="button"
      class="segment {size}"
      class:on={chosen.includes(segment.key)}
      style:--on-bg={segment.accent || "var(--accent-action)"}
      style:--on-fg={segment.ink || "var(--text-on-accent)"}
      aria-pressed={chosen.includes(segment.key)}
      disabled={disabled || segment.disabled}
      onclick={() => onSelect?.(segment.key)}>{segment.label}</button>
  {/each}
</div>

<style>
  .control {
    display: flex;
    border: 1px solid var(--border-default);
  }

  .segment {
    font: var(--machine-sm);
    color: var(--text-muted);
    background: transparent;
    border: 0;
    border-radius: 0;
    cursor: pointer;
  }

  .sm {
    font-size: calc(10.5px * var(--ui-scale));
    padding: calc(4px * var(--ui-scale)) calc(10px * var(--ui-scale));
  }

  .md {
    font-size: calc(11px * var(--ui-scale));
    padding: calc(8px * var(--ui-scale)) calc(12px * var(--ui-scale));
  }

  .segment + .segment {
    border-left: 1px solid var(--border-default);
  }

  .segment:disabled {
    cursor: default;
  }

  .on {
    font-weight: 700;
    color: var(--on-fg);
    background: var(--on-bg);
  }
</style>
