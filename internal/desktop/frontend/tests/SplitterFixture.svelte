<script>
  import { onMount } from "svelte";
  import { createMeasurementController } from "../src/lib/measure.js";
  import Splitter from "../src/lib/shell/Splitter.svelte";
  import { watchSize } from "../src/lib/xterm.js";

  const DEFAULT_WIDTH = 400;
  const STORAGE_KEY = "qrouton.human-pane:splitter-fixture";

  let customWidth = $state(Number(localStorage.getItem(STORAGE_KEY)) || 0);
  let width = $derived(customWidth || DEFAULT_WIDTH);
  let resizeCalls = $state(0);
  let commitCalls = $state(0);
  let resetCalls = $state(0);
  let storageWrites = $state(0);
  let fits = $state(0);
  let sizeHost;

  function resize(next) {
    customWidth = next;
    resizeCalls++;
  }

  function commit(next) {
    resize(next);
    commitCalls++;
    storageWrites++;
    localStorage.setItem(STORAGE_KEY, String(next));
  }

  function reset() {
    customWidth = 0;
    resetCalls++;
    storageWrites++;
    localStorage.removeItem(STORAGE_KEY);
  }

  onMount(() => {
    const measurement = createMeasurementController({ window, document, Storage });
    let stopSize = watchSize(sizeHost, () => fits++);
    window.splitterMetrics = () => ({
      width,
      resizeCalls,
      commitCalls,
      resetCalls,
      storageWrites,
      stored: localStorage.getItem(STORAGE_KEY),
      fits,
    });
    window.resetFits = () => (fits = 0);
    window.measurementSummary = () => measurement.snapshot();
    window.resetMeasurement = () => measurement.reset();
    window.stopMeasurement = () => measurement.stop();
    window.destroyMeasurement = () => measurement.destroy();
    window.sizeBurst = () => {
      window.dispatchEvent(new Event("resize"));
      window.dispatchEvent(new Event("resize"));
      window.dispatchEvent(new Event("resize"));
    };
    window.stopDuringSizeBurst = () => {
      window.dispatchEvent(new Event("resize"));
      stopSize?.();
      stopSize = undefined;
    };
    return () => {
      stopSize?.();
      measurement.destroy();
    };
  });
</script>

<div class="workspace">
  <div class="agent"></div>
  <Splitter
    size={width}
    min={320}
    max={640}
    onResize={resize}
    onCommit={commit}
    onReset={reset}
    label="Resize the shell pane" />
  <div id="pane" style:width="{width}px"></div>
</div>
<div role="separator" aria-label="Resize the shell panes" data-testid="decoy-separator"></div>
<div id="size-host" bind:this={sizeHost}></div>

<style>
  .workspace {
    width: 1000px;
    height: 240px;
    display: flex;
  }

  .agent {
    flex: 1;
    background: #24273a;
  }

  #pane {
    flex: none;
    background: #363a4f;
  }

  #size-host {
    width: 100px;
    height: 100px;
  }
</style>
