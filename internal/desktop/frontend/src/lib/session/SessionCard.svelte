<script>
  import { MODE_LABELS, REPO_ROLES } from "../bridge/generated.js";
  import Button from "../core/Button.svelte";
  import Chip from "../core/Chip.svelte";
  import { GLYPHS, READ_ONLY } from "../roles.js";

  /** @type {{initials?: string, name?: string, mode?: string, lastOpened?: string, description?: string, repos?: {name: string, role: string}[], progress?: {label: string, state: string}[], selected?: boolean, actions?: import('svelte').Snippet, [attribute: string]: any}} */
  let {
    initials,
    name,
    mode = MODE_LABELS.RPI,
    lastOpened,
    description,
    repos = [],
    progress,
    selected = false,
    actions,
    ...rest
  } = $props();

  let modeTone = $derived(mode === MODE_LABELS.RPI ? "guided" : "assistant");
</script>

<div class="card" class:selected {...rest}>
  <div class="initials">{initials}</div>

  <div class="body">
    <div class="heading">
      <span class="name">{name}</span>
      <Chip tone={modeTone} selected>{mode}</Chip>
      {#if lastOpened}<span class="opened">· last opened {lastOpened}</span>{/if}
    </div>
    <div class="description" class:absent={!description}>
      {description || "No description yet."}
    </div>
    {#if repos.length}
      <div class="repos">
        {#each repos as repo (repo.name)}
          <Chip
            tone={repo.role}
            glyph={GLYPHS[repo.role]}
            meta={repo.role === REPO_ROLES.EDITING ? REPO_ROLES.EDITING : READ_ONLY}>{repo.name}</Chip>
        {/each}
      </div>
    {/if}
    {#if progress}
      <div class="progress">
        <span class="progress-label">Progress</span>
        {#each progress as step (step.label)}
          <span class="stage {step.state}">{step.label}{step.state === "done" ? " ✓" : ""}</span>
        {/each}
      </div>
    {/if}
  </div>

  <div class="actions">
    {#if actions}{@render actions()}{:else}
      <Button variant={selected ? "primary" : "outline"}>Open</Button>
    {/if}
  </div>
</div>

<style>
  .card {
    display: flex;
    gap: calc(18px * var(--ui-scale));
    align-items: flex-start;
    padding: calc(16px * var(--ui-scale));
    border: 1px solid var(--border-default);
    background: transparent;
  }

  .selected {
    border-color: var(--accent-action);
    background: var(--surface-raised);
    box-shadow: var(--shadow-focus-lg);
  }

  .initials {
    width: calc(44px * var(--ui-scale));
    height: calc(44px * var(--ui-scale));
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    font: 700 calc(17px * var(--ui-scale)) var(--font-machine);
    background: var(--surface-raised);
    border: 1px solid var(--border-default);
    color: var(--text-secondary);
  }

  .selected .initials {
    background: var(--accent-action);
    border: none;
    color: var(--text-on-accent);
  }

  .body {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: calc(7px * var(--ui-scale));
  }

  .heading {
    display: flex;
    align-items: center;
    gap: calc(10px * var(--ui-scale));
  }

  .name {
    font: var(--display-xs);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  .opened {
    font: var(--machine-sm);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-muted);
  }

  .description {
    font: var(--machine-sm);
    font-size: calc(12px * var(--ui-scale));
    color: var(--text-secondary);
  }

  .description.absent {
    color: var(--text-muted);
  }

  .repos {
    display: flex;
    flex-wrap: wrap;
    gap: calc(8px * var(--ui-scale));
    margin-top: 2px;
  }

  .progress {
    display: flex;
    align-items: center;
    gap: calc(9px * var(--ui-scale));
    margin-top: calc(5px * var(--ui-scale));
  }

  .progress-label {
    font: var(--instruction-sm);
    letter-spacing: var(--instruction-tracking-sm);
    text-transform: uppercase;
    color: var(--text-faint);
  }

  .stage {
    font: var(--machine-bold);
    font-size: calc(10.5px * var(--ui-scale));
    color: var(--text-muted);
    border: 1px solid var(--border-default);
    padding: 1px calc(6px * var(--ui-scale));
  }

  .stage.active {
    color: var(--accent-action);
    border-color: var(--accent-action);
  }

  /* Green means it succeeded; selection is not an achievement. */
  .stage.done {
    color: var(--text-on-accent);
    background: var(--state-success);
    border: none;
    padding: 2px calc(7px * var(--ui-scale));
  }

  .actions {
    flex: none;
    display: flex;
    flex-direction: column;
    gap: calc(8px * var(--ui-scale));
    align-items: stretch;
  }
</style>
