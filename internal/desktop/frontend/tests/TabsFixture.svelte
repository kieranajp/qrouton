<script>
  import TabStrip from "../src/lib/shell/TabStrip.svelte";
  import { reorderWindow, surfaces } from "../src/lib/docked.svelte.js";

  // Fed by the surfaces store the workbench uses, so the tabs under test are
  // the payload Go sends and not a hand-written approximation of it.
  const open = surfaces(() => "fixture");
  let selected = $derived(Math.max(0, open.tabs.findIndex((tab) => tab.id === open.selected)));
</script>

<TabStrip
  tabs={open.tabs}
  {selected}
  onReorder={(from, to) => reorderWindow("fixture", open.tabs[from].id, to)}
  onNew={() => {}}
  newLabel="Shell" />
