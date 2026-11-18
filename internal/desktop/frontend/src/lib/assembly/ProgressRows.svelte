<script>
  import AssemblyStep from "./AssemblyStep.svelte";

  /** @type {{rows: import("./progress.js").Row[]}} */
  let { rows } = $props();
</script>

<div class="progress">
  {#each rows as row, i (i)}
    <AssemblyStep
      state={row.state}
      label={row.label}
      detail={row.detail}
      percent={row.percent} />
  {/each}
</div>

<style>
  .progress {
    display: flex;
    flex-direction: column;
  }
</style>
