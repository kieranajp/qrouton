<script>
  import "../diff.css";

  /** @type {{doc: {text: string, format: string, source: string}, id?: string, active?: boolean, scrollRoot?: HTMLElement}} */
  let { doc, id: _id, active: _active, scrollRoot: _scrollRoot } = $props();

  let lines = $derived(doc.text.split("\n"));
</script>

{#each lines as line, i (i)}
  <div class="diff-row">{line === "" ? " " : line}</div>
{/each}
