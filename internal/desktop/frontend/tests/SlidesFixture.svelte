<script>
  import DockedDocument from "../src/lib/DockedDocument.svelte";
  import PresentOverlay from "../src/lib/panes/PresentOverlay.svelte";
  import { present } from "../src/lib/panes/present.svelte.js";

  let active = $state(true);
  window.deactivate = () => (active = false);
</script>

<DockedDocument id="w1" {active} />

{#if present.active}
  <PresentOverlay />
{/if}
