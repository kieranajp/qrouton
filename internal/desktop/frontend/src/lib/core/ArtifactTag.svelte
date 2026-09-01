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
    font: 700 9px var(--font-machine);
    padding: 2px 6px;
    background: var(--artifact);
    color: var(--ink);
  }

  .long {
    padding: 3px 7px;
  }

  /* A menu reads down its own left edge, so its tags share a width. */
  .fixed {
    width: 52px;
    box-sizing: content-box;
    text-align: center;
  }
</style>
