<script>
  import DocumentPane from "./DocumentPane.svelte";
  import TerminalPane from "./shell/TerminalPane.svelte";

  /** @type {{id: string, active?: boolean}} */
  let { id, active = false } = $props();
</script>

<TerminalPane style="display: {active ? 'flex' : 'none'}">
  <div class="body">
    <DocumentPane {id} />
  </div>
</TerminalPane>

<style>
  .body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }
</style>
