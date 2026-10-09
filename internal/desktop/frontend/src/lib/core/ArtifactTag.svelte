<script>
  import { artifactInk, artifactLabel, artifactTone } from "../artifacts.js";

  /** @type {{kind?: string, id?: string, long?: boolean, fixed?: boolean}} */
  let { kind, id = "", long = false, fixed = false } = $props();

  let text = $derived(artifactLabel(kind, { id, long }));
</script>

<span
  class="tag"
  class:long
  class:fixed
  style:--artifact={artifactTone(kind)}
  style:--ink={artifactInk(kind)}>{text}</span
>

<style>
  .tag {
    flex: none;
    font: 700 calc(9px * var(--ui-scale)) var(--font-machine);
    padding: 2px calc(6px * var(--ui-scale));
    background: var(--artifact);
    color: var(--ink);
  }

  .long {
    padding: calc(3px * var(--ui-scale)) calc(7px * var(--ui-scale));
  }

  /* A menu reads down its own left edge, so its tags share a width. */
  .fixed {
    width: calc(52px * var(--ui-scale));
    box-sizing: content-box;
    text-align: center;
  }
</style>
