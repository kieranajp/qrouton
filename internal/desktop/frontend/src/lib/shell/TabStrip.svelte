<script>
  import Button from "../core/Button.svelte";
  import StatusDot from "../core/StatusDot.svelte";
  import { dismissible } from "../core/dismiss.js";
  import Menu from "./Menu.svelte";
  import Tab from "./Tab.svelte";
  import { dominantStatus, split, tabLabel } from "./tabs.js";

  /** @type {{tabs?: {id?: string, label: string, badge?: string, artifact?: string, status?: 'succeeded'|'running'|'failed'|'waiting'|'idle', closable?: boolean}[], selected?: number, onSelect?: (index: number) => void, onClose?: (index: number) => void, onNew?: () => void, newLabel?: string, [attribute: string]: any}} */
  let { tabs = [], selected = 0, onSelect, onClose, onNew, newLabel = "New ▾", ...rest } = $props();

  // Below this a tab is a coloured rectangle; the rest go to the menu.
  const MIN_TAB = 104;

  let width = $state(0);
  let reserved = $state(0);
  let chip = $state(0);
  let listing = $state(false);

  let room = $derived(Math.max(0, width - reserved));
  // Decided before the chip is measured in: taking a tab off to make room for
  // the chip would otherwise remove the overflow that summoned it.
  let overflowing = $derived(width > 0 && tabs.length > Math.floor(room / MIN_TAB));
  let capacity = $derived(
    overflowing ? Math.max(1, Math.floor((room - chip) / MIN_TAB)) : tabs.length,
  );
  let drawn = $derived(split(tabs, selected, capacity));
  let hiddenStatus = $derived(dominantStatus(drawn.hidden.map(({ tab }) => tab)));

  function reveal(index) {
    listing = false;
    onSelect?.(index);
  }
</script>

<div class="strip" bind:clientWidth={width} {...rest}>
  {#each drawn.shown as { tab, index } (tab.id ?? tab.label)}
    <Tab
      label={tab.label}
      badge={tab.badge}
      artifact={tab.artifact}
      status={tab.status}
      selected={index === selected}
      closable={tab.closable !== false}
      onSelect={() => onSelect?.(index)}
      onClose={() => onClose?.(index)} />
  {/each}
  {#if drawn.hidden.length}
    <span class="more" bind:clientWidth={chip} use:dismissible={() => (listing = false)}>
      <Button
        variant="ghost"
        size="sm"
        onclick={() => (listing = !listing)}
        aria-label="{drawn.hidden.length} more tabs{hiddenStatus ? `, ${hiddenStatus}` : ''}">
        {#if hiddenStatus}
          <StatusDot state={hiddenStatus} size={7} />
        {/if}
        {drawn.hidden.length} ▾
      </Button>
      {#if listing}
        <Menu
          label="Also open"
          width={260}
          align="right"
          offsetY={36}
          items={drawn.hidden.map(({ tab }) => ({ label: tabLabel(tab), status: tab.status }))}
          onSelect={(_, i) => reveal(drawn.hidden[i].index)} />
      {/if}
    </span>
  {/if}
  {#if onNew}
    <span class="new" bind:clientWidth={reserved}>
      <Button variant="dashed" size="sm" glyph="+" onclick={onNew}>{newLabel}</Button>
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
    position: relative;
    z-index: 4;
  }

  .more {
    display: flex;
    align-items: center;
    flex: none;
    padding-left: 4px;
  }

  .new {
    margin-left: auto;
    align-self: center;
    flex: none;
  }
</style>
