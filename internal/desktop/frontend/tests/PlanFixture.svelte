<script>
  import DockedDocument from "../src/lib/DockedDocument.svelte";
</script>

<!-- The docked pane as the workbench mounts it, so the chrome between the
     window edge and the plan's own footer is under test too. -->
<DockedDocument id="w1" active={true} />
