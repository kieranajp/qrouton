<script>
  /** @type {{entries: {label: string, summary?: boolean, color?: string}[], current: number, onSelect: (index: number) => void}} */
  let { entries, current, onSelect } = $props();
</script>

<div class="pips">
  {#each entries as entry, index}
    <button
      type="button"
      class="pip"
      class:summary={entry.summary}
      class:viewing={current === index}
      aria-label={entry.label}
      aria-current={current === index}
      onclick={() => onSelect(index)}>
      <span class="mark" style:background={entry.color ?? null}></span>
    </button>
  {/each}
</div>

<style>
  .pips {
    display: flex;
    flex: none;
    flex-wrap: nowrap;
    gap: calc(6px * var(--ui-scale));
  }

  .pip {
    padding: calc(6px * var(--ui-scale)) calc(3px * var(--ui-scale));
    border: 0;
    border-bottom: 2px solid transparent;
    background: transparent;
    cursor: pointer;
  }

  .pip.viewing {
    border-bottom-color: var(--accent-action);
  }

  .mark {
    display: block;
    width: calc(14px * var(--ui-scale));
    height: calc(5px * var(--ui-scale));
  }

  .summary .mark {
    box-shadow: inset 0 0 0 1px var(--text-faint);
  }

  .summary {
    margin-right: calc(4px * var(--ui-scale));
  }

  :global(html[data-narrow]) .pips {
    flex: 1 0 100%;
  }
</style>
