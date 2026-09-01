<script>
  import "../diff.css";

  /** @type {{doc: {text: string, format: string, source: string}, id?: string, active?: boolean, scrollRoot?: HTMLElement, onScroller?: (element: HTMLElement | null) => void}} */
  let { doc, id: _id, active: _active, scrollRoot: _scrollRoot, onScroller: _onScroller } = $props();

  let lines = $derived(doc.text.split("\n"));
</script>

<div class="plain">
  {#each lines as line, i (i)}
    <div class="diff-row">{line === "" ? " " : line}</div>
  {/each}
</div>

<style>
  .plain {
    padding: 12px 14px;
  }
</style>
