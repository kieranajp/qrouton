<script>
  /** @type {{percent: number}} */
  let { percent } = $props();
</script>

<div class="bar" role="progressbar" aria-valuenow={percent} aria-valuemin="0" aria-valuemax="100">
  <span style:width="{percent}%"></span>
</div>

<style>
  .bar {
    height: calc(4px * var(--ui-scale));
    background: var(--border-subtle);
  }

  span {
    display: block;
    height: 100%;
    background: var(--accent-action);
  }
</style>
