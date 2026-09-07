<script>
  import DockedDocument from "../src/lib/DockedDocument.svelte";
  import PresentOverlay from "../src/lib/panes/PresentOverlay.svelte";
  import { present } from "../src/lib/panes/present.svelte.js";

  let active = $state(true);
  let mounted = $state(true);
  window.deactivate = () => (active = false);
  window.closeTab = () => (mounted = false);
</script>

{#if mounted}
  <DockedDocument id="w1" {active} />
{/if}

{#if present.active}
  <PresentOverlay />
{/if}
