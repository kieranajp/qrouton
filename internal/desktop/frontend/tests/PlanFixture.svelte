<script>
  import DocumentPane from "../src/lib/DocumentPane.svelte";

  /** @type {HTMLElement | undefined} */
  let root = $state();
</script>

<div id="scroll" bind:this={root}>
  {#if root}
    <DocumentPane id="w1" active={true} scrollRoot={root} />
  {/if}
</div>
