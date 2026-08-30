<script>
  import MarkdownPane from "../src/lib/panes/MarkdownPane.svelte";

  const TEXT = Array.from({ length: 40 }, (_, i) => `Paragraph ${i + 1}.`).join("\n\n");

  let doc = $state({
    text: TEXT,
    format: "markdown",
    source: "thoughts/shared/notes/fixture.md",
    line: 1,
    to: 1,
    viewportEpoch: 1,
  });
  let root = $state();

  // The push the workbench makes when the file changes under an open pane: the
  // body stays mounted and the epoch the workbench fences reports against moves.
  window.pushEpoch = (epoch) => (doc = { ...doc, viewportEpoch: epoch });
  window.scrollTo_ = (top) => (root.scrollTop = top);
</script>

<div class="root" bind:this={root}>
  <MarkdownPane {doc} id="w1" active={true} scrollRoot={root} bare />
</div>

<style>
  .root {
    height: 180px;
    width: 420px;
    overflow-y: auto;
  }
</style>
