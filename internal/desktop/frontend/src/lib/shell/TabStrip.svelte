<script>
  import Button from "../core/Button.svelte";
  import Tab from "./Tab.svelte";

  /** @type {{tabs?: {id?: string, label: string, status?: 'succeeded'|'success'|'running'|'failed'|'waiting'|'idle', closable?: boolean}[], selected?: number, onSelect?: (index: number) => void, onClose?: (index: number) => void, onNew?: () => void, newLabel?: string, [attribute: string]: any}} */
  let { tabs = [], selected = 0, onSelect, onClose, onNew, newLabel = "New ▾", ...rest } = $props();
</script>

<div class="strip" {...rest}>
  {#each tabs as tab, i (tab.id ?? tab.label)}
    <Tab
      label={tab.label}
      status={tab.status}
      selected={i === selected}
      closable={tab.closable !== false}
      onSelect={() => onSelect?.(i)}
      onClose={() => onClose?.(i)} />
  {/each}
  {#if onNew}
    <span class="new">
      <Button variant="outline" size="sm" glyph="+" onclick={onNew}>{newLabel}</Button>
    </span>
  {/if}
</div>

<style>
  .strip {
    height: var(--h-pane-header);
    flex: none;
    display: flex;
    align-items: stretch;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--surface-chrome);
    padding-right: 8px;
  }

  .new {
    margin-left: auto;
    align-self: center;
  }
</style>
