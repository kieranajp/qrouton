<script>
  import StatusDot from "../core/StatusDot.svelte";

  // An unfocused tab that cannot report a red test run is one you must click to
  // trust, so the process's state rides along with the label.
  /** @type {{label?: string, status?: 'succeeded'|'success'|'running'|'failed'|'waiting'|'idle', selected?: boolean, closable?: boolean, onSelect?: () => void, onClose?: () => void, [attribute: string]: any}} */
  let { label, status, selected = false, closable = true, onSelect, onClose, ...rest } = $props();
</script>

<div class="tab" class:selected {...rest}>
  <button type="button" class="select" onclick={onSelect}>
    {#if status}<StatusDot state={status === "succeeded" ? "success" : status} size={7} />{/if}
    <span class="label">{label}</span>
  </button>
  {#if closable}
    <button type="button" class="close" aria-label="Close tab" onclick={() => onClose?.()}
      >&#10005;</button>
  {/if}
</div>

<style>
  .tab {
    display: flex;
    align-items: stretch;
    gap: 9px;
    padding: 0 14px;
    border-right: 1px solid var(--border-subtle);
    border-bottom: 2px solid transparent;
    background: transparent;
  }

  /* Selection is blue and separate from status. */
  .selected {
    border-bottom-color: var(--accent-action);
    background: var(--wash-selected);
  }

  .select,
  .close {
    display: flex;
    align-items: center;
    background: none;
    border: 0;
    padding: 0;
    font: inherit;
    color: inherit;
    text-align: inherit;
    cursor: pointer;
  }

  .select {
    gap: 9px;
  }

  .label {
    font: var(--machine-sm);
    font-size: 11.5px;
    color: var(--text-muted);
  }

  .selected .label {
    color: var(--text-primary);
  }

  .close {
    font-size: 11px;
    color: var(--text-faint);
  }
</style>
