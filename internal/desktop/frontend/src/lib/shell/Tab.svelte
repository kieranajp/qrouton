<script>
  import StatusDot from "../core/StatusDot.svelte";
  import { tabLabel } from "./tabs.js";

  // An unfocused tab that cannot report a red test run is one you must click to
  // trust, so the process's state rides along with the label.
  /** @type {{label?: string, badge?: string, status?: 'succeeded'|'success'|'running'|'failed'|'waiting'|'idle', selected?: boolean, closable?: boolean, onSelect?: () => void, onClose?: () => void, [attribute: string]: any}} */
  let { label, badge, status, selected = false, closable = true, onSelect, onClose, ...rest } = $props();

  let whole = $derived(tabLabel({ badge, label }));
</script>

<div class="tab" class:selected title={whole} {...rest}>
  <button type="button" class="select" onclick={onSelect}>
    {#if status}<StatusDot state={status === "succeeded" ? "success" : status} size={7} />{/if}
    <span class="label">{#if badge}<span class="badge">{badge}</span>{/if}{label}</span>
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
    flex: 1 1 auto;
    min-width: 0;
    max-width: 210px;
    overflow: hidden;
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
    min-width: 0;
    flex: 1 1 auto;
  }

  .label {
    font: var(--machine-sm);
    font-size: 11.5px;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .selected .label {
    color: var(--text-primary);
  }

  /* A badge is a plan's id, so it wears the plan artifact's colour whether the
     tab is selected or not. */
  .badge {
    margin-right: 0.5ch;
    color: var(--artifact-plan);
  }

  .close {
    font-size: 11px;
    color: var(--text-faint);
  }
</style>
