<script>
  import Button from "../core/Button.svelte";
  import Chip from "../core/Chip.svelte";
  import TextField from "../forms/TextField.svelte";

  /** @type {{orgs?: string[], orgInput?: string, root?: string, editor?: string, launch?: string, fields?: Record<string, string>, restartRequired?: boolean, onAddOrg?: () => void, onRemoveOrg?: (org: string) => void, onQuit?: () => void}} */
  let {
    orgs = [],
    orgInput = $bindable(""),
    root = $bindable(""),
    editor = $bindable(""),
    launch = $bindable(""),
    fields = {},
    restartRequired = false,
    onAddOrg,
    onRemoveOrg,
    onQuit,
  } = $props();
</script>

<div class="heading">
  <span class="title">Settings</span>
  <span class="helper">Written to config.json.</span>
</div>

<TextField label="GitHub orgs" bind:value={orgInput} placeholder="org-name">
  {#snippet trailing()}
    <Button variant="secondary" onclick={onAddOrg}>Add</Button>
  {/snippet}
</TextField>
{#if orgs.length}
  <div class="orgs">
    {#each orgs as org (org)}
      <span class="org">
        <Chip>{org}</Chip>
        <Button variant="ghost" size="sm" aria-label="Remove {org}" onclick={() => onRemoveOrg?.(org)}
          >×</Button>
      </span>
    {/each}
  </div>
{/if}

<TextField
  label="Sessions root"
  bind:value={root}
  help={fields.root ?? "Takes effect for sessions started after a restart"}
  helpTone={fields.root ? "failed" : "muted"} />

<TextField
  label="Editor"
  bind:value={editor}
  valueVoice="literal"
  help={fields.editor ?? "One {} placeholder for the file path"}
  helpTone={fields.editor ? "failed" : "muted"} />

<TextField
  label="Launch overrides"
  multiline
  bind:value={launch}
  valueVoice="literal"
  help={fields.launch ?? "JSON, keyed by runner id"}
  helpTone={fields.launch ? "failed" : "muted"} />

{#if restartRequired}
  <div class="banner">
    <span>Quit qrouton to use the new sessions root</span>
    <Button variant="secondary" onclick={onQuit}>Quit qrouton</Button>
  </div>
{/if}

<style>
  .heading {
    display: flex;
    align-items: flex-end;
    gap: 18px;
  }

  .title {
    font: var(--display-md);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  .helper {
    flex: 1;
    font: var(--machine-sm);
    color: var(--text-muted);
    padding-bottom: 5px;
  }

  .orgs {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .org {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }

  .banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    padding: 10px 12px;
    background: var(--surface-chrome);
    border: 1px solid var(--border-subtle);
    font: var(--machine-sm);
    color: var(--text-secondary);
  }
</style>
