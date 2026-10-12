<script>
  /** @type {{title: string, rows: {term: string, meaning: string}[]}} */
  let { title, rows } = $props();
</script>

<dl class="legend">
  <dt class="caption">{title}</dt>
  {#each rows as row (row.term)}
    <dt class="term">{row.term}</dt>
    <dd>{row.meaning}</dd>
  {/each}
</dl>

<style>
  .legend {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    column-gap: calc(14px * var(--ui-scale));
    margin: 0;
    border-bottom: 1px solid var(--border-subtle);
  }

  .caption {
    grid-column: 1 / -1;
    padding-bottom: calc(8px * var(--ui-scale));
    font: var(--instruction-sm);
    letter-spacing: var(--instruction-tracking-sm);
    text-transform: uppercase;
    color: var(--text-faint);
  }

  .term,
  dd {
    margin: 0;
    padding: calc(7px * var(--ui-scale)) 0;
    border-top: 1px solid var(--border-subtle);
  }

  .term {
    font: var(--literal);
    color: var(--accent-label);
  }

  dd {
    font: var(--machine-sm);
    color: var(--text-secondary);
  }
</style>
