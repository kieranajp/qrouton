<script>
  import DocumentPane from "../src/lib/DocumentPane.svelte";

  /** @type {HTMLElement | undefined} */
  let root = $state();
</script>

<!-- The scroll port and its wrapper as DockedDocument builds them, so the pane
     is laid out here the way it is in the workbench. -->
<div id="scroll" bind:this={root} class="body">
  <div>
    {#if root}
      <DocumentPane id="w1" active={true} scrollRoot={root} />
    {/if}
  </div>
</div>

<style>
  .body > div {
    display: flex;
    flex-direction: column;
    min-height: 100%;
  }
</style>
