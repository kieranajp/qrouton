<script>
  import { ARTIFACT_KINDS } from "../bridge/generated.js";
  import CapsLabel from "../core/CapsLabel.svelte";
  import CubeMark from "../core/CubeMark.svelte";
  import { artifactTone } from "../artifacts.js";
  import CopyPath from "./CopyPath.svelte";

  /** @type {{source?: string, path?: string, kind?: string, face?: string, tag: import('svelte').Snippet}} */
  let { source, path, kind, face, tag } = $props();
</script>

<div class="head">
  <CubeMark size={18} face={face ?? artifactTone(kind)} data-artifact-kind={kind ?? ARTIFACT_KINDS.NOTE} />
  {@render tag()}
  {#if source}
    <CapsLabel tone="dim" title={source}>{source}</CapsLabel>
  {/if}
  <CopyPath {path} />
</div>

<style>
  .head {
    display: flex;
    align-items: center;
    gap: calc(12px * var(--ui-scale));
    padding: calc(26px * var(--ui-scale)) var(--pane-pad, calc(34px * var(--ui-scale))) calc(20px * var(--ui-scale)) calc(var(--pane-pad, calc(34px * var(--ui-scale))) + var(--gutter));
  }

  .head :global(.caps) {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .head :global(.chip),
  .head :global(.button) {
    flex-shrink: 0;
  }
</style>
