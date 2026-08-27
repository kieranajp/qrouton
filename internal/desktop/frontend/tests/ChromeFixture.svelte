<script>
  import { chrome } from "../src/lib/chrome.svelte.js";

  const current = chrome();
  window.chromeState = () => ({ fields: current.fields, settled: current.settled });
</script>

<output>{current.fields.activity}</output>
