<script>
  import { workflowTone } from "../workflow.js";

  /** @type {{stages?: {research?: boolean, plan?: boolean, implement?: boolean}}} */
  let { stages = {} } = $props();

  // Implement writes no document of its own, so it wears the success colour
  // rather than an artifact's.
  let marks = $derived([
    { letter: "R", done: Boolean(stages.research), tone: workflowTone("RESEARCH"), state: "written" },
    { letter: "P", done: Boolean(stages.plan), tone: workflowTone("PLAN"), state: "written" },
    { letter: "I", done: Boolean(stages.implement), tone: workflowTone("IMPLEMENT"), state: "complete" },
  ]);

  const said = (mark) => `${mark.letter}: ${mark.done ? mark.state : `not ${mark.state} yet`}`;
</script>

<div class="marks" aria-label="Workflow stages">
  {#each marks as mark (mark.letter)}
    <span class="mark" class:done={mark.done} aria-label={said(mark)}>
      <!-- Filled or hollow, never part-filled: a progress bar would imply a
           fraction qrouton cannot observe. -->
      <span class="square" style:--tone={mark.tone} aria-hidden="true"></span>
      {mark.letter}
    </span>
  {/each}
</div>

<style>
  .marks {
    display: flex;
    align-items: center;
    gap: 14px;
    flex: none;
  }

  .mark {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    font: var(--machine-sm);
    font-size: 11px;
    color: var(--text-faint);
  }

  .done {
    color: var(--text-primary);
  }

  .square {
    width: 9px;
    height: 9px;
    box-shadow: inset 0 0 0 1px var(--ctp-surface-2);
  }

  .done .square {
    background: var(--tone);
    box-shadow: none;
  }
</style>
