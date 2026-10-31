<script>
  import Button from "../core/Button.svelte";
  import ArtifactHeader from "./ArtifactHeader.svelte";
  import ReaderFooter from "./ReaderFooter.svelte";

  /** @type {{doc: {source: string, path?: string, kind?: string}, structured: string,
   * label: string, mode: string, onMode: (mode: string) => void, tag: any, body: any,
   * controls?: any, bar?: any, counter?: any, pips?: boolean}} */
  let { doc, structured, label, mode, onMode, tag, body, controls, bar, counter, pips = false } = $props();
</script>

<article class="document">
  <ArtifactHeader source={doc.source} path={doc.path} kind={doc.kind} {tag} />
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
    --pane-pad: calc(34px * var(--ui-scale));
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
  }

</style>
