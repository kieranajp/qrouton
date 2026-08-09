<script>
  import { onMount } from "svelte";
  import TerminalPane from "./shell/TerminalPane.svelte";
  import "./diff.css";
  import { diffClass } from "./diff.js";
  import { Call } from "./wails.js";

  const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

  /** @type {{id: string, active?: boolean}} */
  let { id, active = false } = $props();

  /** @type {string[]} */
  let lines = $state([]);
  let format = $state("");

  onMount(async () => {
    const doc = await Call.ByName(WINDOWS_SERVICE + ".Content", id);
    format = doc.format;
    lines = doc.text.split("\n");
  });
</script>

<TerminalPane style="display: {active ? 'flex' : 'none'}">
  <!-- The window declares its format; guessing it from the text would paint a
       plain document that quotes a diff as one. -->
  <div class="body">
    {#each lines as line, i (i)}
      <div class="row {format === 'diff' ? diffClass(line) : ''}">{line === "" ? " " : line}</div>
    {/each}
  </div>
</TerminalPane>

<style>
  .body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }
</style>
