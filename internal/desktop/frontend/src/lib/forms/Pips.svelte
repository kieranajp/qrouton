<script>
  import { pipCounter, pipStates } from "./pips.js";

  /** @type {{total?: number, active?: number, [attribute: string]: any}} */
  let { total = 0, active = 0, ...rest } = $props();

  let states = $derived(pipStates(total, active));
  let counter = $derived(pipCounter(total, active));
</script>

<div class="pips" {...rest}>
  <div class="row">
    {#each states as state, i (i)}
      <span class="pip {state}"></span>
    {/each}
  </div>
  <span class="counter">{counter}</span>
</div>

<style>
  .pips {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 7px;
  }

  .pip {
    width: 8px;
    height: 8px;
    flex: none;
  }

  .pip.on {
    width: 18px;
    background: var(--accent-action);
  }

  /* A screen already answered has no state worth a hue, the same grey an idle
     session's dot uses. */
  .pip.done {
    background: var(--ctp-surface-2);
  }

  .pip.todo {
    background: var(--surface-raised);
  }

  .counter {
    font: var(--machine-sm);
    font-size: 10.5px;
    color: var(--text-faint);
  }
</style>
