<script>
  import { onMount, tick } from "svelte";
  import { paneFor } from "./panes/index.js";
  import { Call, Events } from "./wails.js";

  const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

  /** @type {{id: string, active?: boolean, scrollRoot?: HTMLElement, onReady?: () => void}} */
  let { id, active = false, scrollRoot, onReady } = $props();

  /** @type {{text: string, format: string, source: string, path?: string, kind?: string, line: number, to: number, viewportEpoch?: number} | undefined} */
  let doc = $state();

  // The window follows its file, so the pane is told about a write it did not
  // make. A pull resolving after a push must not put the older text back.
  let live = false;
  onMount(() => {
    const off = Events.On("window:content:" + id, (event) => {
      if (!event?.data) return;
      live = true;
      doc = event.data;
    });
    (async () => {
      const content = await Call.ByName(WINDOWS_SERVICE + ".Content", id);
      if (!live) doc = content;
      await tick();
      onReady?.();
    })();
    return off;
  });
</script>

{#if doc}
  {@const Pane = paneFor(doc.format, doc.kind)}
  <Pane {doc} {id} {active} {scrollRoot} />
{/if}
