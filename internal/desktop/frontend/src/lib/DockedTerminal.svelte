<script>
  import { onDestroy, onMount } from "svelte";
  import TerminalPane from "./shell/TerminalPane.svelte";
  import { createTerminalActivation } from "./terminal-focus.js";
  import { Call, Events } from "./wails.js";
  import { decode, encode, mount, watchSize } from "./xterm.js";

  const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

  /** @type {{id: string, active?: boolean, focus?: number, focusPending?: boolean, onFocused?: (generation: number) => void}} */
  let { id, active = false, focus = 0, focusPending = false, onFocused } = $props();

  let host;
  let term = $state();
  let teardown;
  let fit;

  const activation = createTerminalActivation({
    frame: requestAnimationFrame,
    cancelFrame: cancelAnimationFrame,
    refit: () => fit?.(),
    focus: () => term?.focus(),
    handled: (generation) => onFocused?.(generation),
  });

  $effect(() => activation.update(active, focus, focusPending));

  // An async onMount callback returns a Promise, not a cleanup Svelte would
  // call, so teardown is set here and onDestroy reads it instead.
  onMount(() => {
    (async () => {
      const write = (text) => Call.ByName(WINDOWS_SERVICE + ".Write", id, encode(text));
      const started = mount(host, { write, background: "--ctp-crust" });
      const { refit, dispose } = started;
      term = started.term;
      const resize = (cols, rows) => Call.ByName(WINDOWS_SERVICE + ".Resize", id, cols, rows);
      fit = () => refit(resize);

      term.onBinary((data) => Call.ByName(WINDOWS_SERVICE + ".Write", id, btoa(data)));
      const offData = Events.On("window:data:" + id, (event) => term.write(decode(event.data)));
      const offExit = Events.On("window:exit:" + id, (event) => {
        term.write("\r\n\x1b[2m[exited with status " + event.data + "]\x1b[0m\r\n");
      });
      const stopWatch = watchSize(host, fit);

      teardown = () => {
        offData();
        offExit();
        stopWatch();
        dispose();
      };

      await Call.ByName(WINDOWS_SERVICE + ".Start", id, term.cols, term.rows);
    })();
  });

  onDestroy(() => {
    activation.destroy();
    teardown?.();
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
