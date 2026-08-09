<script>
  import StatusDot from "../core/StatusDot.svelte";

  // An unfocused tab that cannot report a red test run is one you must click to
  // trust, so the process's state rides along with the label.
  /** @type {{label?: string, status?: 'succeeded'|'success'|'running'|'failed'|'waiting'|'idle', selected?: boolean, closable?: boolean, onSelect?: () => void, onClose?: () => void, [attribute: string]: any}} */
  let { label, status, selected = false, closable = true, onSelect, onClose, ...rest } = $props();
</script>

<div class="tab" class:selected role="presentation" onclick={onSelect} {...rest}>
  {#if status}<StatusDot state={status === "succeeded" ? "success" : status} size={7} />{/if}
  <span class="label">{label}</span>
  {#if closable}
    <span
      class="close"
      role="presentation"
      onclick={(event) => {
        event.stopPropagation();
        onClose?.();
      }}>&#10005;</span>
  {/if}
</div>

<style>
  .tab {
    display: flex;
    align-items: center;
    gap: 9px;
    padding: 0 14px;
    cursor: pointer;
    border-right: 1px solid var(--border-subtle);
    border-bottom: 2px solid transparent;
    background: transparent;
  }

  /* Selection is blue and separate from status. */
  .selected {
    border-bottom-color: var(--accent-action);
    background: var(--wash-selected);
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
