<script>
  import { onMount, tick } from "svelte";
  import { paneFor } from "./panes/index.js";
  import { Call } from "./wails.js";

  const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

  /** @type {{id: string, active?: boolean, scrollRoot?: HTMLElement, onReady?: () => void}} */
  let { id, active = false, scrollRoot, onReady } = $props();

  /** @type {{text: string, format: string, source: string, path?: string, kind?: string, line: number, to: number, viewportEpoch?: number} | undefined} */
  let doc = $state();

  onMount(async () => {
    doc = await Call.ByName(WINDOWS_SERVICE + ".Content", id);
    await tick();
    onReady?.();
  });
</script>

{#if doc}
  {@const Pane = paneFor(doc.format)}
  <Pane {doc} {id} {active} {scrollRoot} />
{/if}
