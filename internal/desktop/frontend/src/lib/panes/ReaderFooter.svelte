<script>
  /** @type {{bar?: any, navigation?: any, actions?: any, counter?: any, pips?: boolean}} */
  let { bar, navigation, actions, counter, pips = false } = $props();
</script>

<footer class="footer" class:with-pips={pips}>
  {@render bar?.()}
  <div class="controls">
    {@render navigation?.()}
    <div class="modes">{@render actions?.()}</div>
    {@render counter?.()}
  </div>
</footer>

<style>
  .footer {
    flex: none;
    margin-top: auto;
    background: var(--surface-chrome);
    border-top: var(--border-width) solid var(--border-subtle);
  }

  .controls {
    display: flex;
    align-items: center;
    gap: 14px;
    min-height: var(--h-footer);
    padding: 0 var(--pane-pad);
  }

  .modes {
    display: flex;
    flex: none;
    gap: 6px;
    margin-left: auto;
  }

  .with-pips .modes {
    margin-left: 0;
  }

  .footer :global(.counter) {
    flex: 1 1 0;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: right;
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .footer :global(.steps) {
    display: flex;
    flex: none;
    gap: 6px;
  }

  @media (max-width: 420px) {
    .with-pips .controls {
      flex-wrap: wrap;
      row-gap: 10px;
      padding-top: 8px;
      padding-bottom: 8px;
    }
  }
</style>
