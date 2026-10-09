<script>
  import Button from "../core/Button.svelte";
  import TextField from "../forms/TextField.svelte";
  import { movedPaths } from "./thoughts.js";

  /** @type {{thoughts?: import("./thoughts.js").ThoughtsForm, loaded?: import("./thoughts.js").ThoughtsForm & {derived?: string}, error?: string, onAdd?: () => void, onRemove?: (index: number) => void}} */
  let {
    thoughts = $bindable({ default: "", roots: [] }),
    loaded = { default: "", roots: [], derived: "" },
    error = "",
    onAdd,
    onRemove,
  } = $props();

  let moved = $derived(movedPaths(loaded, thoughts));
</script>

<TextField
  label="Default thoughts folder"
  aria-label="Default thoughts folder"
  bind:value={thoughts.default}
  placeholder={loaded.derived}
  valueVoice="literal"
  help="Sessions write research, specs and plans here unless a shared folder below claims them" />

<fieldset class="shared">
  <legend>Shared thoughts folders</legend>
  <p>
    A session whose repositories all belong to one folder's orgs writes its documents there, not in
    the default folder. qrouton doesn't sync anything. A folder is shared only if Syncthing,
    Dropbox or similar syncs it. A session that mixes orgs, or has no repositories, stays in the
    default folder.
  </p>
  {#each thoughts.roots as row, index (index)}
    <div class="row">
      <TextField
        label="Name"
        aria-label="Shared folder name"
        bind:value={row.id}
        readonly={row.saved}
        valueVoice="literal" />
      <TextField
        label="Folder"
        aria-label="Shared folder path"
        bind:value={row.path}
        valueVoice="literal"
        help="If this folder is missing, sessions go to the default folder until it exists." />
      <TextField
        label="Orgs"
        aria-label="Shared folder orgs"
        bind:value={row.orgs}
        placeholder="org, other-org" />
      <Button
        variant="ghost"
        size="sm"
        aria-label="Remove {row.id || 'shared folder'}"
        onclick={() => onRemove?.(index)}>×</Button>
    </div>
  {/each}
  <div>
    <Button variant="secondary" onclick={onAdd}>Add shared folder</Button>
  </div>
</fieldset>

{#if error}
  <p class="note failed">{error}</p>
{/if}
{#if moved}
  <p class="note">
    Sessions already using the old folder keep their documents there, and search won't find them
    until you move them. qrouton doesn't move anything.
  </p>
{/if}

<style>
  .shared {
    display: flex;
    flex-direction: column;
    gap: calc(14px * var(--ui-scale));
    margin: 0;
    border: 0;
    padding: 0;
  }

  .shared legend {
    margin-bottom: calc(10px * var(--ui-scale));
    padding: 0;
    font: var(--display-sm);
    color: var(--text-primary);
  }

  .shared p,
  .note {
    margin: 0;
    font: var(--machine-sm);
    color: var(--text-secondary);
  }

  .note.failed {
    color: var(--state-failed);
  }

  .row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 2fr) minmax(0, 1.5fr) auto;
    align-items: start;
    gap: calc(14px * var(--ui-scale)) calc(18px * var(--ui-scale));
  }
</style>
