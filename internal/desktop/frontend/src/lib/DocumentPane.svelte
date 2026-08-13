<script>
  import { onMount } from "svelte";
  import { paneFor } from "./panes/index.js";
  import { Call } from "./wails.js";

  const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

  /** @type {{id: string}} */
  let { id } = $props();

  /** @type {{text: string, format: string, source: string, line: number, to: number} | undefined} */
  let doc = $state();

  onMount(async () => {
    doc = await Call.ByName(WINDOWS_SERVICE + ".Content", id);
  });
</script>

{#if doc}
  {@const Pane = paneFor(doc.format)}
  <Pane {doc} />
{/if}
