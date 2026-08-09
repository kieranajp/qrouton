<script>
  /** @type {{steps?: string[], active?: number, [attribute: string]: any}} */
  let { steps = [], active = 0, ...rest } = $props();
</script>

<div class="stepper" {...rest}>
  {#each steps as step, i (step)}
    {#if i}<div class="rule"></div>{/if}
    <div class="step">
      <div class="marker" class:on={i === active} class:done={i < active}>
        {i < active ? "✓" : i + 1}
      </div>
      <span class="label" class:on={i === active}>{step}</span>
    </div>
  {/each}
</div>

<style>
  .stepper {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .rule {
    width: 26px;
    height: 1px;
    background: var(--border-default);
  }

  .step {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .marker {
    width: 20px;
    height: 20px;
    display: flex;
    align-items: center;
    justify-content: center;
    font: var(--button);
    font-size: 11px;
    background: transparent;
    border: 1px solid var(--border-default);
    color: var(--text-muted);
  }

  .marker.done {
    background: var(--surface-raised);
    border: none;
  }

  .marker.on {
    background: var(--accent-action);
    border: none;
    color: var(--text-on-accent);
  }

  .label {
    font: var(--instruction-sm);
    letter-spacing: var(--instruction-tracking-sm);
    text-transform: uppercase;
    color: var(--text-faint);
  }

  .label.on {
    font: var(--instruction);
    letter-spacing: var(--instruction-tracking);
    color: var(--accent-label);
  }
</style>
