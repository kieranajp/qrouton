<script>
  import { onDestroy, onMount } from "svelte";
  import TerminalPane from "../shell/TerminalPane.svelte";
  import { latestPerFrame } from "../frame.js";
  import { onScale } from "../scale.svelte.js";
  import { createTerminalActivation } from "../terminal-focus.js";
  import { Events } from "../wails.js";
  import { resizePTY, startPTY, writePTY } from "./calls.js";
  import { createTerminalPainter, encode, fontsReady, mount, terminalFontSize, watchSize } from "../xterm.js";

  /** @type {{id: string, pty: import("./services.js").PTY, active?: boolean,
   *   focus?: number, focusPending?: boolean, focusEligible?: boolean, onFocused?: (generation: number) => void}} */
  let { id, pty, active = false, focus = 0, focusPending = false, focusEligible = true, onFocused } = $props();

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
    if (term) activation.update(active, focus, focusPending, focusEligible);
  });

  // An async onMount callback returns a Promise, not a cleanup Svelte would
  // call, so teardown is set here and onDestroy reads it instead.
  onMount(() => {
    (async () => {
      await fontsReady();
      if (!live) return;
      const write = (text) => writePTY(pty, id, encode(text));
      const started = mount(host, { write, background: "--surface-terminal", terminal: id, pane: pty.pane });
      const { refit, dispose } = started;
      term = started.term;
      const painter = createTerminalPainter(term);
      fit = () => refit((cols, rows) => resizePTY(pty, id, cols, rows));

      term.onBinary((data) => writePTY(pty, id, btoa(data)));
      const offData = Events.On(pty.data + id, (event) => painter.paint(event.data));
      const dim = (line) => painter.paint({ encoded: encode("\r\n\x1b[2m[" + line + "]\x1b[0m\r\n") });
      const offExit = Events.On(pty.exit + id, (event) => dim("exited with status " + event.data));
      const stopWatch = watchSize(host, fit);
      // xterm re-measures its cell after the option changes, so a fit in the same frame reads the old cell.
      const refitScaled = latestPerFrame(() => fit());
      const offScale = onScale((percent) => {
        term.options.fontSize = terminalFontSize(percent);
        refitScaled.schedule();
      });

      teardown = () => {
        offScale();
        refitScaled.cancel();
        offData();
        offExit();
        stopWatch();
        painter.dispose();
        dispose();
      };

      const spawned = await startPTY(pty, id, term.cols, term.rows);
      if (!spawned.ok && live) dim("could not start: " + (spawned.error?.message ?? spawned.error));
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
  <div class="host" data-terminal-id={id} bind:this={host}></div>
</TerminalPane>

<style>
  .host {
    flex: 1;
    min-height: 0;
  }
</style>
