<script>
  import { onDestroy, tick } from "svelte";
  import { WINDOW_CONTENT_EVENT, DOCUMENT_FORMATS } from "./bridge/generated.js";
  import { paneFor } from "./panes/index.js";
  import { freshContent } from "./panes/spec-calls.js";
  import { follow } from "./wails.js";

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

  /** @type {{text: string, hash?: string, format: string, source: string, path?: string, kind?: string, deck?: boolean, assetToken?: string, line: number, to: number, viewportEpoch?: number, assets?: {source: string, path?: string, url: string, kind: import("./bridge/generated.js").MediaKind}[], currentIndex?: number, revision?: number} | undefined} */
  let doc = $state();
  let failure = $state("");
  let windowID = "";
  let stop = () => {};

  $effect(() => {
    const nextID = id;
    if (nextID === windowID) return;
    stop();
    windowID = nextID;
    let stopped = false;
    doc = undefined;
    failure = "";
    const accept = (incoming) => {
      if (!incoming) return;
      if (incoming.format === DOCUMENT_FORMATS.ASSETS && doc?.format === DOCUMENT_FORMATS.ASSETS &&
          (incoming.revision ?? 0) <= (doc.revision ?? 0)) return;
      doc = incoming;
    };
    const following = follow(WINDOW_CONTENT_EVENT + nextID, () => freshContent(nextID), accept, {
      accepts: (data) => Boolean(data),
      settle: async (answer, live) => {
        if (!answer.ok) failure = doc ? "" : String(answer.error?.message ?? answer.error);
        else if (answer.value?.format === DOCUMENT_FORMATS.ASSETS) accept(answer.value);
        else doc = live && doc ? { ...answer.value, text: doc.text, hash: doc.hash } : answer.value;
        await tick();
        if (!stopped) onReady?.();
      },
    });
    stop = () => {
      stopped = true;
      following.stop();
    };
  });

  onDestroy(() => stop());
</script>

{#if doc}
  {@const Pane = paneFor(doc)}
  <Pane {...{slug}} {doc} {id} {active} {scrollRoot} {agentWorking} {onScroller} {onFindAdapter} />
{:else if failure}
  <p class="failed" role="alert">Could not read this document: {failure}</p>
{/if}

<style>
  .failed {
    margin: var(--space-12);
    color: var(--state-failed);
    font: var(--machine-sm);
  }
</style>
