<script>
  import { onMount, tick } from "svelte";
  import { WINDOW_CONTENT_EVENT, WINDOWS_CONTENT } from "./bridge/generated.js";
  import { paneFor } from "./panes/index.js";
  import { Call, Events } from "./wails.js";

  /** @type {{id: string, active?: boolean, scrollRoot?: HTMLElement, agentWorking?: boolean, onReady?: () => void, onScroller?: (element: HTMLElement | null) => void}} */
  let { id, active = false, scrollRoot, agentWorking = false, onReady, onScroller } = $props();

  /** @type {{text: string, format: string, source: string, path?: string, kind?: string, line: number, to: number, viewportEpoch?: number} | undefined} */
  let doc = $state();

  // The window follows its file, so the pane is told about a write it did not
  // make. A push that beats the load keeps its text, but the load's viewport
  // epoch still stands: it is the one the workbench is fencing reports against.
  let live = false;
  onMount(() => {
    const off = Events.On(WINDOW_CONTENT_EVENT + id, (event) => {
      if (!event?.data) return;
      live = true;
      doc = event.data;
    });
    (async () => {
      const content = await Call.ByName(WINDOWS_CONTENT, id);
      doc = live && doc ? { ...content, text: doc.text } : content;
      await tick();
      onReady?.();
    })();
    return off;
  });
</script>

{#if doc}
  {@const Pane = paneFor(doc.format, doc.kind)}
  <Pane {doc} {id} {active} {scrollRoot} {agentWorking} {onScroller} />
{/if}
