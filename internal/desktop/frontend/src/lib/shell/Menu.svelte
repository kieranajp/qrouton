<script>
  import ArtifactTag from "../core/ArtifactTag.svelte";
  import Icon from "../core/Icon.svelte";
  import { TAB_STATUSES } from "../bridge/generated.js";

  /** maxHeight caps a menu whose items are not a fixed vocabulary, which then
   * scrolls rather than running off the screen.
   * @type {{header?: import('svelte').Snippet, label?: string, items?: import("./menu.js").MenuItem[], width?: number, maxHeight?: number, align?: 'left'|'right', offsetY?: number, onSelect?: (item: any, index: number) => void, [attribute: string]: any}} */
  let {
    header,
    label,
    items = [],
    width = 212,
    maxHeight = 0,
    align = "left",
    offsetY = 32,
    onSelect,
    ...rest
  } = $props();

  let marked = $derived(items.some((item) => item !== "-" && (item.icon || (item.status && item.status !== TAB_STATUSES.IDLE))));
</script>

<div
  class="menu"
  class:capped={maxHeight > 0}
  style:width="calc({width}px * var(--ui-scale))"
  style:max-height={maxHeight > 0 ? `calc(${maxHeight}px * var(--ui-scale))` : null}
  style:top="calc({offsetY}px * var(--ui-scale))"
  style:left={align === "left" ? "0" : "auto"}
  style:right={align === "right" ? "0" : "auto"}
  {...rest}>
  {#if header}<div class="header">{@render header()}</div>{/if}
  {#if label}<div class="heading">{label}</div>{/if}
  {#each items as item, i (i)}
    {#if item === "-"}
      <div class="rule"></div>
    {:else if item.heading}
      <div class="heading">{item.heading}</div>
    {:else}
      <div class:branch={item.items?.length}>
        <button
          class="item"
          class:active={item.active}
          class:destructive={item.tone === "destructive"}
          disabled={item.disabled}
          aria-haspopup={item.items?.length ? "menu" : undefined}
          onclick={item.items?.length ? undefined : () => onSelect?.(item, i)}>
          {#if item.icon}
            <Icon name={item.icon} />
          {:else if item.status && item.status !== TAB_STATUSES.IDLE}
            <Icon name={item.status} />
          {:else if marked}
            <span class="slot"></span>
          {/if}
          {#if item.tag}
            <ArtifactTag kind={item.tag} id={item.id ?? ""} fixed />
          {/if}
          <span class="label">{item.label}</span>
          {#if item.meta}<span class="meta">{item.meta}</span>{/if}
          {#if item.items?.length}<span class="submenu-caret"><Icon name="submenu" /></span>{/if}
        </button>
        {#if item.items?.length}
          <div class="submenu" style:width="calc({item.width ?? width}px * var(--ui-scale))">
            {#each item.items as child, childIndex (childIndex)}
              <button class="item" onclick={() => onSelect?.(child, childIndex)}>
                {#if child.tag}
                  <ArtifactTag kind={child.tag} id={child.id ?? ""} fixed />
                {/if}
                <span class="label">{child.label}</span>
                {#if child.meta}<span class="meta">{child.meta}</span>{/if}
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  {/each}
</div>

<style>
  .menu {
    position: absolute;
    background: var(--surface-chrome);
    border: 1px solid var(--accent-action);
    box-shadow: var(--shadow-menu);
    display: flex;
    flex-direction: column;
    padding: calc(5px * var(--ui-scale)) 0;
    z-index: 5;
  }

  .capped {
    box-sizing: border-box;
    overflow: hidden auto;
  }

  .header {
    position: sticky;
    top: calc(-5px * var(--ui-scale));
    z-index: 1;
    flex: none;
    padding: calc(7px * var(--ui-scale)) calc(12px * var(--ui-scale));
    background: var(--surface-chrome);
  }

  .heading {
    padding: calc(7px * var(--ui-scale)) calc(12px * var(--ui-scale)) calc(8px * var(--ui-scale));
    font: var(--instruction-sm);
    letter-spacing: var(--instruction-tracking-sm);
    text-transform: uppercase;
    color: var(--text-faint);
  }

  .rule {
    height: 1px;
    background: var(--border-subtle);
    margin: calc(5px * var(--ui-scale)) 0;
  }

  .item {
    padding: calc(8px * var(--ui-scale)) calc(12px * var(--ui-scale));
    display: flex;
    align-items: center;
    gap: calc(10px * var(--ui-scale));
    cursor: pointer;
    background: transparent;
    border: 0;
    text-align: left;
    width: 100%;
    font: var(--machine-sm);
    color: var(--text-secondary);
  }

  .item.active,
  .item:enabled:hover {
    background: var(--surface-raised);
    color: var(--text-primary);
  }

  .item.destructive,
  .item.destructive:enabled:hover {
    color: var(--action-destructive);
  }

  .item.destructive:enabled:hover {
    background: color-mix(in srgb, var(--action-destructive) 14%, var(--surface-raised));
  }

  .item:disabled {
    color: var(--text-faint);
    cursor: default;
  }

  .branch {
    position: relative;
  }

  .submenu {
    position: absolute;
    top: calc(-6px * var(--ui-scale));
    left: calc(100% - 1px);
    padding: calc(5px * var(--ui-scale)) 0;
    background: var(--surface-chrome);
    border: 1px solid var(--accent-action);
    box-shadow: var(--shadow-menu);
    visibility: hidden;
    opacity: 0;
    pointer-events: none;
  }

  .branch:hover > .submenu,
  .branch:focus-within > .submenu {
    visibility: visible;
    opacity: 1;
    pointer-events: auto;
  }

  .submenu-caret {
    color: var(--text-muted);
    flex: none;
  }

  .slot {
    width: calc(16px * var(--ui-scale));
    flex: none;
  }

  .label {
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .meta {
    font: var(--machine-xs);
    font-size: calc(10px * var(--ui-scale));
    color: var(--text-faint);
    flex: none;
  }
</style>
