<script>
  /** @type {Record<string, string>} */
  const TONES = {
    success: "var(--state-success)",
    waiting: "var(--state-waiting)",
    faint: "var(--text-faint)",
  };

  /** @type {{shown: {text: string, tone: string}}} */
  let { shown } = $props();
</script>

<span class="badge" style:--tone={TONES[shown.tone]}><span class="dot"></span>{shown.text}</span>

<style>
  .badge {
    display: flex;
    align-items: center;
    gap: calc(8px * var(--ui-scale));
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font: var(--machine-xs);
    color: var(--text-secondary);
  }

  .dot {
    width: calc(7px * var(--ui-scale));
    height: calc(7px * var(--ui-scale));
    flex: none;
    background: var(--tone);
  }
</style>
