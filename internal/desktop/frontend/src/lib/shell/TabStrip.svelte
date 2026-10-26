<script>
  import Button from "../core/Button.svelte";
  import Icon from "../core/Icon.svelte";
  import { TAB_STATUSES } from "../bridge/generated.js";
  import { dismissible } from "../core/dismiss.js";
  import Menu from "./Menu.svelte";
  import Tab from "./Tab.svelte";
  import { uiScale } from "../scale.svelte.js";
  import { dominantStatus, dropIndex, fits, need, split, tabLabel } from "./tabs.js";

  /** @type {{tabs?: {id?: string, label: string, badge?: string, artifact?: string, icon?: import("../bridge/generated.js").WindowIcon, status?: import("../bridge/generated.js").TabStatus, closable?: boolean, pinned?: boolean}[], selected?: number, onSelect?: (index: number) => void, onClose?: (index: number) => void, onReorder?: (from: number, to: number) => void, onNew?: () => void, newLabel?: string, [attribute: string]: any}} */
  let {
    tabs = [],
    selected = 0,
    onSelect,
    onClose,
    onReorder,
    onNew,
    newLabel = "New",
    ...rest
  } = $props();

  let width = $state(0);
  let reserved = $state(0);
  let chip = $state(0);
  let home = $state(0);
  let listing = $state(false);
  /** @type {HTMLElement | undefined} */
  let strip = $state();

  // The pinned tab is sized to its label, so it is measured rather than estimated.
  $effect(() => {
    void tabs;
    void width;
    home = strip?.querySelector(".home")?.getBoundingClientRect().width ?? 0;
  });

  // A tab that cannot keep a readable label goes to the menu.
  let widths = $derived(tabs.map((tab) => (tab.pinned ? home : (need(tab) * uiScale()) / 100)));
  let room = $derived(Math.max(0, width - reserved));
  // Decided before the chip is measured in: taking a tab off to make room for
  // the chip would otherwise remove the overflow that summoned it.
  let overflowing = $derived(width > 0 && fits(widths, room) < tabs.length);
  let capacity = $derived(overflowing ? Math.max(1, fits(widths, room - chip)) : tabs.length);
  let drawn = $derived(split(tabs, selected, capacity));
  let hiddenStatus = $derived(dominantStatus(drawn.hidden.map(({ tab }) => tab)));

  function reveal(index) {
    listing = false;
    onSelect?.(index);
  }

  // Held by key, not position: a tab that closes mid-drag never reports the drag
  // ending, and a stale index would name whichever tab moved up into it.
  let lifted = $state("");
  let hovered = $state("");
  const key = (tab) => tab.id ?? tab.label;

  function settle() {
    lifted = "";
    hovered = "";
  }

  function dropped(index) {
    const from = drawn.shown.find((entry) => key(entry.tab) === lifted)?.index ?? -1;
    settle();
    if (from < 0) return;
    if (tabs[index]?.pinned) return;
    const to = dropIndex(drawn.shown, from, index);
    if (to !== from) onReorder?.(from, to);
  }
</script>

<div class="strip" bind:this={strip} bind:clientWidth={width} {...rest}>
  {#each drawn.shown as { tab, index } (tab.id ?? tab.label)}
    <Tab
      label={tab.label}
      badge={tab.badge}
      artifact={tab.artifact}
      icon={tab.icon}
      status={tab.status}
      selected={index === selected}
      closable={tab.closable !== false && !tab.pinned}
      pinned={tab.pinned}
      dragging={key(tab) === lifted}
      over={key(tab) === hovered && key(tab) !== lifted}
      onSelect={() => onSelect?.(index)}
      onClose={() => onClose?.(index)}
      onDragStart={() => (lifted = key(tab))}
      onDragOver={() => (hovered = key(tab))}
      onDragLeave={() => hovered === key(tab) && (hovered = "")}
      onDrop={() => dropped(index)}
      onDragEnd={settle} />
  {/each}
  {#if drawn.hidden.length}
    <span class="more" bind:clientWidth={chip} use:dismissible={() => (listing = false)}>
      <Button
        variant="ghost"
        size="sm"
        onclick={() => (listing = !listing)}
        aria-label="{drawn.hidden.length} more tabs{hiddenStatus ? `, ${hiddenStatus}` : ''}">
        {#if hiddenStatus && hiddenStatus !== TAB_STATUSES.IDLE}<Icon name={hiddenStatus} />{/if}
        {drawn.hidden.length}
        <Icon name="overflow" />
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
      <Button variant="cube" size="sm" icon="new" onclick={onNew}>{newLabel}</Button>
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
    padding-right: calc(8px * var(--ui-scale));
    position: relative;
    z-index: 4;
  }

  .more {
    display: flex;
    align-items: center;
    flex: none;
    padding-left: calc(4px * var(--ui-scale));
  }

  /* Three pixels off the right edge, so the cube's shadow has somewhere to fall. */
  .new {
    margin-left: auto;
    align-self: center;
    flex: none;
    padding-right: calc(3px * var(--ui-scale));
  }
</style>
