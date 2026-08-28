<script>
  import { onMount } from "svelte";
  import ContextMenu from "../src/lib/shell/ContextMenu.svelte";
  import { mount } from "../src/lib/xterm.js";

  let host;
  let field = $state("");

  onMount(() => {
    window.written = [];
    const started = mount(host, { write: (text) => window.written.push(text) });
    window.selectTerminal = () => started.term.selectAll();
    window.terminalSelection = () => started.term.getSelection();
    started.term.write("output");

    // Registered after the menu's own listener, so it reads the verdict.
    window.defaults = { prevented: 0, allowed: 0 };
    window.addEventListener("contextmenu", (event) => {
      window.defaults[event.defaultPrevented ? "prevented" : "allowed"]++;
    });
    return () => started.dispose();
  });
</script>

<div class="page">
  <input id="field" aria-label="Field" bind:value={field} />
  <p id="prose">Some prose to select.</p>
  <a id="link" href="https://example.com/doc">A document</a>
  <div id="chrome">Inert chrome</div>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div id="claimed" oncontextmenu={(event) => event.preventDefault()}>Claims its own click</div>
  <div id="terminal" bind:this={host}></div>
</div>

<ContextMenu />

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 8px;
  }

  #terminal {
    height: 160px;
  }
</style>
