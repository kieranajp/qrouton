<script>
  import { tick } from "svelte";
  import { WINDOW_CONTENT_EVENT, WINDOWS_CONTENT } from "./bridge/generated.js";
  import { paneFor } from "./panes/index.js";
  import { Call, Events } from "./wails.js";

  /** @type {{id: string, slug?: string, active?: boolean, scrollRoot?: HTMLElement, agentWorking?: boolean, onReady?: () => void, onScroller?: (element: HTMLElement | null) => void, onFindAdapter?: (adapter: import("./find.js").FindAdapter | null) => void}} */
  let {
    id,
    slug = "",
    active = false,
    scrollRoot,
    agentWorking = false,
    onReady,
    onScroller,
    onFindAdapter,
  } = $props();

  /** @type {{text: string, format: string, source: string, path?: string, kind?: string, deck?: boolean, assetToken?: string, line: number, to: number, viewportEpoch?: number, images?: {source: string, url: string}[], currentIndex?: number, revision?: number} | undefined} */
  let doc = $state();

  $effect(() => {
    const windowID = id;
    let disposed = false;
    let live = false;
    doc = undefined;
    const accept = (incoming) => {
      if (disposed || !incoming) return;
      if (incoming.format === "images" && doc?.format === "images" &&
          (incoming.revision ?? 0) <= (doc.revision ?? 0)) return;
      doc = incoming;
    };
    const off = Events.On(WINDOW_CONTENT_EVENT + windowID, (event) => {
      if (!event?.data || disposed) return;
      live = true;
      accept(event.data);
    });
    (async () => {
      const content = await Call.ByName(WINDOWS_CONTENT, windowID);
      if (disposed) return;
      if (content?.format === "images") accept(content);
      else doc = live && doc ? { ...content, text: doc.text } : content;
      await tick();
      if (!disposed) onReady?.();
    })();
    return () => {
      disposed = true;
      off();
    };
  });
</script>

{#if doc}
  {@const Pane = paneFor(doc)}
  <Pane {...{slug}} {doc} {id} {active} {scrollRoot} {agentWorking} {onScroller} {onFindAdapter} />
{/if}
