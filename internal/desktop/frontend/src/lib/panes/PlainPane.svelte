<script>
  import "../diff.css";

  /** @type {{doc: {text: string, format: string, source: string}}} */
  let { doc } = $props();

  let lines = $derived(doc.text.split("\n"));
</script>

{#each lines as line, i (i)}
  <div class="diff-row">{line === "" ? " " : line}</div>
{/each}
