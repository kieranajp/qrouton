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
    gap: 6px;
  }

  .pip {
    padding: 6px 3px;
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
    width: 14px;
    height: 5px;
  }

  .summary .mark {
    box-shadow: inset 0 0 0 1px var(--text-faint);
  }

  .summary {
    margin-right: 4px;
  }

  @media (max-width: 420px) {
    .pips {
      flex: 1 0 100%;
    }
  }
</style>
