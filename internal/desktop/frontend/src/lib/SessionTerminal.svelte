<script>
  import { onDestroy, onMount } from "svelte";
  import TerminalPane from "./shell/TerminalPane.svelte";
  import { Call, Events } from "./wails.js";
  import { decode, encode, mount, watchSize } from "./xterm.js";

  const TERM_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Term";

  /** @type {{id: string, active?: boolean, focus?: number}} */
  let { id, active = false, focus = 0 } = $props();

  let host;
  let term = $state();
  let teardown;
  let sized;

  // A session switch is a surface the user asked for, so it takes the keyboard.
  // The pane has only just stopped being display:none, and an element with no
  // layout box can be neither measured nor focused — hence the frame's wait.
  $effect(() => {
    if (!active || !term) return;
    // focus is a token, read so that a change re-runs this: an overlay closing
    // is what bumps it, and the keyboard belongs back here after one.
    void focus;
    const frame = requestAnimationFrame(() => {
      sized?.();
      term.focus();
    });
    return () => cancelAnimationFrame(frame);
  });

  // An async onMount callback returns a Promise, not a cleanup Svelte would
  // call, so teardown is set here and onDestroy reads it instead.
  onMount(() => {
    (async () => {
      const write = (text) => Call.ByName(TERM_SERVICE + ".Write", id, encode(text));
      const started = mount(host, { write, background: "--ctp-crust" });
      const { refit, dispose } = started;

      term = started.term;
      term.onBinary((data) => Call.ByName(TERM_SERVICE + ".Write", id, btoa(data)));
      const offData = Events.On("pty:data:" + id, (event) => term.write(decode(event.data)));
      // A supervisor that failed keeps its terminal, so this is read after it.
      const offExit = Events.On("pty:exit:" + id, (event) => {
        term.write("\r\n\x1b[2m[session ended — status " + event.data + "]\x1b[0m\r\n");
      });
      sized = () => refit((cols, rows) => Call.ByName(TERM_SERVICE + ".Resize", id, cols, rows));
      const stopWatch = watchSize(host, sized);

      teardown = () => {
        offData();
        offExit();
        stopWatch();
        dispose();
      };

      await Call.ByName(TERM_SERVICE + ".Start", id, term.cols, term.rows);
    })();
  });

  onDestroy(() => teardown?.());
</script>

<!-- Hidden rather than unmounted: a session you switch away from keeps working,
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
