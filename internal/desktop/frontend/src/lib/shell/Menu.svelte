<script>
  import { artifactTone } from "../artifacts.js";
  import StatusDot from "../core/StatusDot.svelte";

  /** @type {{label?: string, items?: any[], width?: number, align?: 'left'|'right', offsetY?: number, onSelect?: (item: any, index: number) => void, [attribute: string]: any}} */
  let { label, items = [], width = 212, align = "left", offsetY = 32, onSelect, ...rest } = $props();
</script>

<div
  class="menu"
  style:width="{width}px"
  style:top="{offsetY}px"
  style:left={align === "left" ? "0" : "auto"}
  style:right={align === "right" ? "0" : "auto"}
  {...rest}>
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
          {#if item.status}
            <StatusDot state={item.status === "succeeded" ? "success" : item.status} size={7} />
          {/if}
          {#if item.tag}
            <span class="tag" style:--artifact={artifactTone(item.tag)}>{item.tag}</span>
          {/if}
          <span class="label">{item.label}</span>
          {#if item.meta}<span class="meta">{item.meta}</span>{/if}
          {#if item.items?.length}<span class="submenu-caret">&#8250;</span>{/if}
        </button>
        {#if item.items?.length}
          <div class="submenu" style:width="{item.width ?? width}px">
            {#each item.items as child, childIndex (childIndex)}
              <button class="item" onclick={() => onSelect?.(child, childIndex)}>
                {#if child.tag}
                  <span class="tag" style:--artifact={artifactTone(child.tag)}>{child.tag}</span>
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
    padding: 5px 0;
    z-index: 5;
  }

  .heading {
    padding: 7px 12px 8px;
    font: var(--instruction-sm);
    letter-spacing: var(--instruction-tracking-sm);
    text-transform: uppercase;
    color: var(--text-faint);
  }

  .rule {
    height: 1px;
    background: var(--border-subtle);
    margin: 5px 0;
  }

  .item {
    padding: 8px 12px;
    display: flex;
    align-items: center;
    gap: 10px;
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
    top: -6px;
    left: calc(100% - 1px);
    padding: 5px 0;
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
    font-size: 16px;
    line-height: 0;
    flex: none;
  }

  .tag {
    font: 700 9px var(--font-machine);
    color: var(--text-on-accent);
    background: var(--artifact);
    padding: 2px 6px;
    width: 52px;
    text-align: center;
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
    font-size: 10px;
    color: var(--text-faint);
    flex: none;
  }
</style>
