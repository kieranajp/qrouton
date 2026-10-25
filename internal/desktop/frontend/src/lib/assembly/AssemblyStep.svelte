<script>
  import Icon from "../core/Icon.svelte";

  /** @type {Record<"pending"|"running"|"done"|"failed", import("../core/icons.js").IconName>} */
  const ICONS = { done: "succeeded", running: "running", failed: "failed", pending: "pending" };

  /** @type {{state?: 'pending'|'running'|'done'|'failed', label?: string, detail?: string, percent?: number, [attribute: string]: any}} */
  let { state = "pending", label, detail, percent, ...rest } = $props();
</script>

<div class="step" {...rest}>
  <Icon name={ICONS[state]} />
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
    gap: calc(12px * var(--ui-scale));
    padding: calc(8px * var(--ui-scale)) 0;
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
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-faint);
  }

  .track {
    width: calc(190px * var(--ui-scale));
    height: calc(8px * var(--ui-scale));
    flex: none;
    background: var(--surface-raised);
  }

  .fill {
    height: calc(8px * var(--ui-scale));
    background: var(--accent-action);
  }

  .percent {
    font: var(--terminal-sm);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-muted);
    width: calc(38px * var(--ui-scale));
    text-align: right;
  }
</style>
