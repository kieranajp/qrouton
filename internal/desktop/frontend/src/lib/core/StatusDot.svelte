<script>
  const TONES = {
    selected: "var(--state-selected)",
    succeeded: "var(--state-success)",
    running: "var(--state-running)",
    failed: "var(--state-failed)",
    waiting: "var(--state-waiting)",
    guided: "var(--state-guided)",
    idle: "var(--ctp-surface-2)",
  };

  /** @type {{state?: 'selected'|'succeeded'|'running'|'failed'|'waiting'|'guided'|'idle', size?: number, [attribute: string]: any}} */
  let { state = "running", size = 9, ...rest } = $props();
</script>

<span
  aria-hidden="true"
  class="dot"
  style:width="{size}px"
  style:height="{size}px"
  style:background={TONES[state]}
  {...rest}
></span>

<style>
  .dot {
    display: inline-block;
    flex: none;
  }
</style>
