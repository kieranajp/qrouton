<script>
  /** @type {{label?: string, windows?: {id?: string, label: string, kind: string}[], right?: string, onClose?: (window: any) => void, [attribute: string]: any}} */
  let { label = "Agent windows", windows = [], right, onClose, ...rest } = $props();
</script>

<div class="tray" {...rest}>
  <span class="label">{label}</span>
  {#each windows as window (window.id ?? window.label)}
    <div class="window">
      <span class="name">{window.kind === "document" ? "◆" : "▶"} {window.label}</span>
      <button type="button" class="close" aria-label="Close window" onclick={() => onClose?.(window)}
        >&#10005;</button>
    </div>
  {/each}
  {#if right}<span class="right">{right}</span>{/if}
</div>

<style>
  .tray {
    height: var(--h-tray);
    flex: none;
    background: var(--surface-chrome);
    border-top: 1px solid var(--border-subtle);
    display: flex;
    align-items: center;
    padding: 0 16px;
    gap: 10px;
  }

  .label {
    font: var(--instruction-sm);
    letter-spacing: var(--instruction-tracking-sm);
    text-transform: uppercase;
    color: var(--text-faint);
  }

  .window {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 10px;
    border: 1px solid var(--border-default);
  }

  .name {
    font: var(--machine-sm);
    font-size: 11px;
    color: var(--text-secondary);
  }

  .close {
    font: var(--machine-xs);
    font-size: 10.5px;
    color: var(--text-faint);
    cursor: pointer;
    background: none;
    border: 0;
    padding: 0;
  }

  .right {
    margin-left: auto;
    font: var(--machine-md);
    font-size: 11px;
    color: var(--text-muted);
  }
</style>
