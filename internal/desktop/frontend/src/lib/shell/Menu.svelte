<script>
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
    {:else}
      <button class="item" class:active={item.active} onclick={() => onSelect?.(item, i)}>
        {#if item.status}
          <StatusDot state={item.status === "succeeded" ? "success" : item.status} size={7} />
        {/if}
        {#if item.tag}
          <span class="tag" class:plan={item.tag === "PLAN"}>{item.tag}</span>
        {/if}
        <span class="label">{item.label}</span>
        {#if item.meta}<span class="meta">{item.meta}</span>{/if}
      </button>
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
  .item:hover {
    background: var(--surface-raised);
    color: var(--text-primary);
  }

  .tag {
    font: 700 9px var(--font-machine);
    color: var(--text-on-accent);
    background: var(--accent-label);
    padding: 2px 6px;
    width: 52px;
    text-align: center;
    flex: none;
  }

  .tag.plan {
    background: var(--state-guided);
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
