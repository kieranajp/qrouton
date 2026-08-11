<script>
  import "../diff.css";
  import { diffClass } from "../diff.js";

  /** @type {{doc: {text: string, format: string, source: string}}} */
  let { doc } = $props();

  let lines = $derived(doc.text.split("\n"));
</script>

{#each lines as line, i (i)}
  <div class="diff-row {diffClass(line)}">{line === "" ? " " : line}</div>
{/each}
