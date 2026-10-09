<script>
  import CubeMark from "../core/CubeMark.svelte";

  const COLUMNS = [
    {
      swatch: "var(--accent-label)",
      title: "You pick the repos",
      body: "Search across your GitHub orgs. No cloning by hand, no juggling paths.",
    },
    {
      swatch: "var(--accent-action)",
      title: "qrouton sets it up",
      body: "Mirrors, worktrees, branches and agent instructions, in one folder.",
    },
    {
      swatch: "var(--state-success)",
      title: "The agent gets to work",
      body: "Claude Code, Codex or OpenCode, in a window beside your own terminal.",
    },
  ];
</script>

<div class="lede">
  <CubeMark size={86} />
  <div class="words">
    <h1>qrouton</h1>
    <p>
      Most work touches more than one repository. qrouton checks the relevant ones out together, on
      matching branches, and gives a coding agent a workspace where all of them are already in front
      of it.
    </p>
  </div>
</div>

<div class="rule"></div>

<div class="columns">
  {#each COLUMNS as column (column.title)}
    <div class="column">
      <span class="swatch" style:background={column.swatch}></span>
      <span class="title">{column.title}</span>
      <p>{column.body}</p>
    </div>
  {/each}
</div>

<p class="foot">Three short screens explain the ideas, then two questions and you are set up.</p>

<style>
  .lede {
    display: flex;
    align-items: flex-start;
    gap: calc(26px * var(--ui-scale));
  }

  .words {
    display: flex;
    flex-direction: column;
    gap: calc(12px * var(--ui-scale));
  }

  h1 {
    margin: 0;
    font: var(--display-lg);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  p {
    margin: 0;
    font: var(--machine-md);
    color: var(--text-secondary);
    max-width: 62ch;
  }

  .rule {
    height: 1px;
    background: var(--border-subtle);
  }

  .columns {
    display: flex;
    gap: calc(34px * var(--ui-scale));
  }

  .column {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: calc(10px * var(--ui-scale));
  }

  .swatch {
    width: calc(26px * var(--ui-scale));
    height: calc(26px * var(--ui-scale));
  }

  .title {
    font: var(--display-xs);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  .foot {
    margin: 0;
    font: var(--machine-sm);
    color: var(--text-faint);
  }
</style>
