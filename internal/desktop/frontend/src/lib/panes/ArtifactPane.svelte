<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import CubeMark from "../core/CubeMark.svelte";
  import { artifactTone } from "../artifacts.js";
  import CopyPath from "./CopyPath.svelte";
  import ReaderFooter from "./ReaderFooter.svelte";

  /** @type {{doc: {source: string, path?: string, kind?: string}, structured: string,
   * label: string, mode: string, onMode: (mode: string) => void, tag: any, body: any,
   * controls?: any, bar?: any, counter?: any, pips?: boolean}} */
  let { doc, structured, label, mode, onMode, tag, body, controls, bar, counter, pips = false } = $props();
</script>

<article class="document">
  <div class="head">
    <CubeMark size={18} face={artifactTone(doc.kind)} data-artifact-kind={doc.kind ?? "NOTE"} />
    {@render tag()}
    {#if doc.source}
      <CapsLabel tone="dim">{doc.source}</CapsLabel>
    {/if}
    <CopyPath path={doc.path} />
  </div>
  {@render body()}
  <ReaderFooter {bar} navigation={controls} {counter} {pips}>
    {#snippet actions()}
        <Button
          variant={mode === structured ? "outline" : "ghost"}
          size="sm"
          aria-pressed={mode === structured}
          onclick={() => onMode(structured)}>{label}</Button>
        <Button
          variant={mode === "document" ? "outline" : "ghost"}
          size="sm"
          aria-pressed={mode === "document"}
          onclick={() => onMode("document")}>Document</Button>
    {/snippet}
  </ReaderFooter>
</article>

<style>
  .document {
    --pane-pad: 34px;
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
  }

  /* Aligned with the body's text column rather than the pane edge, so the
     mark, the chip and the path sit over the body's own left margin. */
  .head {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 26px var(--pane-pad) 20px calc(var(--pane-pad) + var(--gutter));
  }

  .head :global(.caps) {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

</style>
