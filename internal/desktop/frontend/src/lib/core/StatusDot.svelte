<script>
  import { TAB_STATUSES } from "../bridge/generated.js";
  const TONES = {
    selected: "var(--state-selected)",
    [TAB_STATUSES.SUCCEEDED]: "var(--state-success)",
    [TAB_STATUSES.RUNNING]: "var(--state-running)",
    [TAB_STATUSES.FAILED]: "var(--state-failed)",
    [TAB_STATUSES.WAITING]: "var(--state-waiting)",
    guided: "var(--state-guided)",
    idle: "var(--ctp-surface-2)",
  };

  /** @type {{state?: 'selected'|'succeeded'|'running'|'failed'|'waiting'|'guided'|'idle', size?: number, [attribute: string]: any}} */
  let { state = "running", size = 9, ...rest } = $props();
</script>

<span
  aria-hidden="true"
  class="dot"
  style:width="calc({size}px * var(--ui-scale))"
  style:height="calc({size}px * var(--ui-scale))"
  style:background={TONES[state]}
  {...rest}
></span>

<style>
  .dot {
    display: inline-block;
    flex: none;
  }
</style>
