<script>
  import { onDestroy, onMount } from "svelte";
  import TerminalPane from "../shell/TerminalPane.svelte";
  import { createTerminalActivation } from "../terminal-focus.js";
  import { Call, Events } from "../wails.js";
  import { encode, fontsReady, mount, paint, watchSize } from "../xterm.js";

  /** @type {{id: string, pty: import("./services.js").PTY, active?: boolean,
   *   focus?: number, focusPending?: boolean, onFocused?: (generation: number) => void}} */
  let { id, pty, active = false, focus = 0, focusPending = false, onFocused } = $props();

  let host;
  let term = $state();
  let teardown;
  let fit;
  let live = true;

  const activation = createTerminalActivation({
    frame: requestAnimationFrame,
    cancelFrame: cancelAnimationFrame,
    refit: () => fit?.(),
    redraw: () => term?.refresh(0, term.rows - 1),
    focus: () => term?.focus(),
    handled: (generation) => onFocused?.(generation),
  });

  // Activation acknowledges a focus request as delivered, so it must not run
  // before there is a terminal to deliver one to.
  $effect(() => {
    if (term) activation.update(active, focus, focusPending);
  });

  // An async onMount callback returns a Promise, not a cleanup Svelte would
  // call, so teardown is set here and onDestroy reads it instead.
  onMount(() => {
    (async () => {
      await fontsReady();
      if (!live) return;
      const write = (text) => Call.ByName(pty.write, id, encode(text));
      const started = mount(host, { write, background: "--ctp-crust" });
      const { refit, dispose } = started;
      term = started.term;
      fit = () => refit((cols, rows) => Call.ByName(pty.resize, id, cols, rows));

      term.onBinary((data) => Call.ByName(pty.write, id, btoa(data)));
      const offData = Events.On(pty.data + id, (event) => paint(term, event.data));
      const offExit = Events.On(pty.exit + id, (event) => {
        term.write("\r\n\x1b[2m[exited with status " + event.data + "]\x1b[0m\r\n");
      });
      const stopWatch = watchSize(host, fit);

      teardown = () => {
        offData();
        offExit();
        stopWatch();
        dispose();
      };

      await Call.ByName(pty.start, id, term.cols, term.rows);
    })();
  });

  onDestroy(() => {
    live = false;
    activation.destroy();
    teardown?.();
  });
</script>

<!-- Hidden rather than unmounted: a surface you switch away from keeps running,
     and tearing down its terminal would lose the scrollback with it. -->
<TerminalPane style="display: {active ? 'flex' : 'none'}">
  <div class="host" bind:this={host}></div>
</TerminalPane>

<style>
  .host {
    flex: 1;
    min-height: 0;
  }
</style>
