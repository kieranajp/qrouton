<script>
  const STATES = {
    done: { glyph: "✓", color: "var(--state-success)" },
    running: { glyph: "◌", color: "var(--state-running)" },
    failed: { glyph: "✗", color: "var(--state-failed)" },
    pending: { glyph: "◌", color: "var(--text-faint)" },
  };

  /** @type {{state?: 'pending'|'running'|'done'|'failed', label?: string, detail?: string, percent?: number, [attribute: string]: any}} */
  let { state = "pending", label, detail, percent, ...rest } = $props();
</script>

<div class="step" {...rest}>
  <span class="glyph" aria-hidden="true" style:color={STATES[state].color}>{STATES[state].glyph}</span>
  <span class="label" class:pending={state === "pending"}>{label}</span>
  {#if detail}<span class="detail">{detail}</span>{/if}
  {#if percent !== undefined}
    <div class="track"><div class="fill" style:width="{percent}%"></div></div>
    <span class="percent">{percent}%</span>
  {/if}
</div>

<style>
  .step {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 0;
  }

  .glyph {
    font: var(--machine-md);
    width: 14px;
  }

  .label {
    flex: 1;
    font: var(--machine-bold);
    color: var(--text-primary);
  }

  .label.pending {
    font: var(--machine-md);
    color: var(--text-muted);
  }

  .detail {
    font: var(--machine-sm);
    font-size: 11px;
    color: var(--text-faint);
  }

  .track {
    width: 190px;
    height: 8px;
    flex: none;
    background: var(--surface-raised);
  }

  .fill {
    height: 8px;
    background: var(--accent-action);
  }

  .percent {
    font: var(--terminal-sm);
    font-size: 11px;
    color: var(--text-muted);
    width: 38px;
    text-align: right;
  }
</style>
