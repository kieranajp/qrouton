<script>
  import { onMount } from "svelte";
  import TerminalPane from "./shell/TerminalPane.svelte";
  import { Call, Events } from "./wails.js";
  import { decode, encode, mount, watchSize } from "./xterm.js";

  const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

  /** @type {{id: string, active?: boolean}} */
  let { id, active = false } = $props();

  let host;
  let term = $state();

  $effect(() => {
    if (active) term?.focus();
  });

  onMount(async () => {
    const write = (text) => Call.ByName(WINDOWS_SERVICE + ".Write", id, encode(text));
    const started = mount(host, { write, background: "--ctp-crust" });
    const { refit } = started;
    term = started.term;

    term.onBinary((data) => Call.ByName(WINDOWS_SERVICE + ".Write", id, btoa(data)));
    Events.On("window:data:" + id, (event) => term.write(decode(event.data)));
    Events.On("window:exit:" + id, (event) => {
      term.write("\r\n\x1b[2m[exited with status " + event.data + "]\x1b[0m\r\n");
    });

    watchSize(host, () =>
      refit((cols, rows) => Call.ByName(WINDOWS_SERVICE + ".Resize", id, cols, rows)),
    );
    await Call.ByName(WINDOWS_SERVICE + ".Start", id, term.cols, term.rows);
  });
</script>

<!-- Hidden rather than unmounted: a tab you switch away from keeps running, and
     tearing down its terminal would lose the scrollback with it. -->
<TerminalPane style="display: {active ? 'flex' : 'none'}">
  <div class="host" bind:this={host}></div>
</TerminalPane>

<style>
  .host {
    flex: 1;
    min-height: 0;
  }
</style>
