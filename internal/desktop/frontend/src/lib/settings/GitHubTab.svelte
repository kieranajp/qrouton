<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import { pinnedNote } from "./pinned.js";
  import TabHeader from "./TabHeader.svelte";

  /** @type {{panel: ReturnType<typeof import("./settings.svelte.js").settings>, labelledby?: string}} */
  let { panel, labelledby } = $props();

  let form = $derived(panel.form);
  let help = $derived(panel.fields.orgs ?? (form.pinned.orgs ? pinnedNote("QROUTON_ORGS") : ""));
</script>

<TabHeader title="GitHub" id={labelledby}>
  Orgs and personal accounts whose repositories appear in the picker.
</TabHeader>

<div class="orgs-field">
  <CapsLabel id="orgs-label">Orgs</CapsLabel>
  <div class="add">
    <div class="input">
      <span class="prefix" aria-hidden="true">github.com/</span>
      <input type="text" aria-labelledby="orgs-label" placeholder="org-name" bind:value={panel.orgInput} />
    </div>
    <Button variant="cube" onclick={panel.add}>Add</Button>
  </div>
  {#if help}<span class="help" class:failed={panel.fields.orgs}>{help}</span>{/if}
</div>

{#if form.orgs.length}
  <ul class="rows">
    {#each form.orgs as org (org)}
      <li>
        <span>{org}</span>
        <button type="button" class="remove" aria-label="Remove {org}" onclick={() => panel.remove(org)}>Remove</button>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .orgs-field {
    display: flex;
    flex-direction: column;
    gap: calc(7px * var(--ui-scale));
    max-width: calc(520px * var(--ui-scale));
  }

  .add {
    display: flex;
    gap: calc(8px * var(--ui-scale));
  }

  .input {
    flex: 1;
    display: flex;
    border: 1px solid var(--border-default);
    background: var(--surface-chrome);
  }

  .input:focus-within {
    border-color: var(--accent-action);
    box-shadow: var(--shadow-focus);
  }

  .prefix {
    padding: calc(9px * var(--ui-scale)) 0 calc(9px * var(--ui-scale)) calc(12px * var(--ui-scale));
    font: var(--literal);
    color: var(--text-muted);
  }

  input {
    flex: 1;
    min-width: 0;
    border: 0;
    outline: none;
    background: transparent;
    padding: calc(9px * var(--ui-scale)) calc(12px * var(--ui-scale)) calc(9px * var(--ui-scale)) 0;
    font: var(--literal);
    color: var(--text-primary);
    caret-color: var(--caret);
  }

  input::placeholder {
    color: var(--text-faint);
  }

  .help {
    font: var(--machine-sm);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-muted);
  }

  .help.failed {
    color: var(--state-failed);
  }

  .rows {
    display: flex;
    flex-direction: column;
    max-width: calc(520px * var(--ui-scale));
    margin: 0;
    padding: 0;
    list-style: none;
    border-top: 1px solid var(--border-subtle);
  }

  li {
    display: flex;
    align-items: center;
    padding: calc(9px * var(--ui-scale)) 0;
    border-bottom: 1px solid var(--border-subtle);
    font: var(--machine-sm);
    color: var(--text-primary);
  }

  .remove {
    margin-left: auto;
    border: 0;
    background: none;
    padding: 0;
    font: var(--machine-sm);
    color: var(--text-faint);
    cursor: pointer;
  }

  .remove:hover,
  .remove:focus-visible {
    color: var(--state-failed);
    outline: none;
  }
</style>
