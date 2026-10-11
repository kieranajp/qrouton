<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import TextField from "../forms/TextField.svelte";
  import { pinnedNote } from "./pinned.js";
  import TabHeader from "./TabHeader.svelte";
  import { movedPaths } from "./thoughts.js";

  /** @type {{panel: ReturnType<typeof import("./settings.svelte.js").settings>, labelledby?: string}} */
  let { panel, labelledby } = $props();

  let form = $derived(panel.form);
  let loaded = $derived(panel.loadedThoughts);
  let moved = $derived(movedPaths(loaded, form.thoughts));
  let rootHelp = $derived(
    panel.fields.root ??
      (form.pinned.root
        ? pinnedNote("QROUTON_ROOT")
        : "Each session gets a folder here, with one git worktree per repo. Applies after qrouton restarts."),
  );
</script>

<TabHeader title="Folders" id={labelledby}>
  Where worktrees and documents live. Sessions already running keep their old paths.
</TabHeader>

<div class="paths">
  <TextField
    label="Sessions root"
    aria-label="Sessions root"
    bind:value={form.root}
    valueVoice="literal"
    help={rootHelp}
    helpTone={panel.fields.root ? "failed" : form.pinned.root ? "waiting" : "muted"} />
  <TextField
    label="Default thoughts folder"
    aria-label="Default thoughts folder"
    bind:value={form.thoughts.default}
    placeholder={loaded.derived}
    valueVoice="literal"
    help="Research, specs and plans go here unless a shared folder below claims the session. Leave it empty to use thoughts/ inside the sessions root." />
</div>

<fieldset class="shared">
  <legend><CapsLabel>Shared thoughts folders</CapsLabel></legend>
  <p>
    A session whose repos all belong to one folder's orgs writes there. Point a folder at somewhere a tool
    such as Syncthing or Dropbox syncs, and teammates' agents can read it. qrouton only writes the files.
  </p>
  <div class="table" role="presentation">
    <span class="head">Name</span>
    <span class="head">Folder</span>
    <span class="head">Orgs</span>
    <span></span>
    {#each form.thoughts.roots as row, index (index)}
      <input
        class="cell name"
        aria-label="Shared folder name"
        placeholder="name"
        bind:value={row.id}
        readonly={row.saved} />
      <input class="cell path" aria-label="Shared folder path" placeholder="~/Sync/thoughts" bind:value={row.path} />
      <input class="cell orgs" aria-label="Shared folder orgs" placeholder="org, other-org" bind:value={row.orgs} />
      <button
        type="button"
        class="remove"
        aria-label="Remove {row.id || 'shared folder'}"
        onclick={() => panel.removeFolder(index)}>Remove</button>
    {/each}
  </div>
  <div class="add">
    <Button variant="cube" icon="new" onclick={panel.addFolder}>Add shared folder</Button>
    <span class="aside">A missing folder falls back to the default.</span>
  </div>
</fieldset>

{#if panel.fields.thoughts}
  <p class="note failed">{panel.fields.thoughts}</p>
{/if}
{#if moved}
  <p class="note">
    Sessions already using the old folder keep their documents there, and search won't find them until you
    move them. qrouton doesn't move anything.
  </p>
{/if}

<style>
  .paths {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: calc(18px * var(--ui-scale));
    align-items: start;
  }

  .shared {
    display: flex;
    flex-direction: column;
    gap: calc(10px * var(--ui-scale));
    margin: 0;
    border: 0;
    padding: calc(6px * var(--ui-scale)) 0 0;
  }

  legend {
    padding: 0;
  }

  .shared p,
  .note {
    margin: 0;
    max-width: calc(640px * var(--ui-scale));
    font: var(--machine-sm);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-muted);
    text-wrap: pretty;
  }

  .note.failed {
    color: var(--state-failed);
  }

  .table {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 2fr) minmax(0, 1.5fr) calc(56px * var(--ui-scale));
    border-top: 1px solid var(--border-subtle);
  }

  .head {
    padding: calc(9px * var(--ui-scale)) 0;
    font: var(--instruction-sm);
    letter-spacing: var(--instruction-tracking-sm);
    text-transform: uppercase;
    color: var(--text-faint);
  }

  .cell,
  .remove {
    min-width: 0;
    border: 0;
    border-top: 1px solid var(--border-subtle);
    background: transparent;
    padding: calc(9px * var(--ui-scale)) calc(12px * var(--ui-scale)) calc(9px * var(--ui-scale)) 0;
  }

  .remove {
    padding-right: 0;
  }

  .cell {
    outline: none;
    font: var(--literal);
    color: var(--text-secondary);
    caret-color: var(--caret);
  }

  .cell.name {
    color: var(--text-muted);
  }

  .cell.orgs {
    font: var(--machine-sm);
    color: var(--text-primary);
  }

  .cell::placeholder {
    color: var(--text-faint);
  }

  .cell:not([readonly]):hover {
    background: var(--wash-selected);
  }

  .cell:not([readonly]):focus {
    background: var(--surface-chrome);
    box-shadow: inset 0 calc(-1px * var(--ui-scale)) 0 var(--accent-action);
  }

  .cell[readonly] {
    cursor: default;
  }

  .remove {
    text-align: right;
    font: var(--machine-sm);
    color: var(--text-faint);
    cursor: pointer;
  }

  .remove:hover,
  .remove:focus-visible {
    color: var(--state-failed);
    outline: none;
  }

  .add {
    display: flex;
    align-items: center;
    gap: calc(16px * var(--ui-scale));
    padding-top: calc(4px * var(--ui-scale));
  }

  .aside {
    font: var(--machine-xs);
    color: var(--text-muted);
  }
</style>
