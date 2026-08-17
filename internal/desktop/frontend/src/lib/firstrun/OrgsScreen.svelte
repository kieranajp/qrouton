<script>
  import Chip from "../core/Chip.svelte";

  /** @type {{orgs?: string[], orgInput?: string, login?: string, onAddOrg?: () => void, onRemoveOrg?: (org: string) => void}} */
  let {
    orgs = [],
    orgInput = $bindable(""),
    login = "",
    onAddOrg,
    onRemoveOrg,
  } = $props();

  // Enter belongs to this field rather than to the dialog: the chips are the only
  // way to answer the question, and there is no Add button beside them.
  function key(event) {
    if (event.key !== "Enter") return;
    event.preventDefault();
    event.stopPropagation();
    onAddOrg?.();
  }
</script>

<h1>Whose repositories should I search?</h1>

<p>
  GitHub organisations or personal accounts. qrouton lists everything you can see under them, so you
  can find repos by name instead of remembering URLs.
</p>

<div class="field">
  {#each orgs as org (org)}
    <Chip selected>
      {org}
      <button class="remove" aria-label="Remove {org}" onclick={() => onRemoveOrg?.(org)}>✕</button>
    </Chip>
  {/each}
  <input
    type="text"
    bind:value={orgInput}
    onkeydown={key}
    placeholder="Add an org or username…" />
</div>

{#if login}
  <p class="help">
    Signed in as <span class="login">{login}</span> via the GitHub CLI. You can add more later in
    Settings.
  </p>
{:else}
  <p class="help">
    Not signed in to the GitHub CLI — run <code>gh auth login</code> before qrouton can search these
    owners. You can add more later in Settings.
  </p>
{/if}

<p class="foot">
  Repos outside these owners still work — name one on the command line and qrouton will resolve it.
</p>

<style>
  h1 {
    margin: 0;
    font: var(--display-md);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  p {
    margin: 0;
    font: var(--machine-md);
    color: var(--text-secondary);
    max-width: 78ch;
  }

  .field {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    background: var(--surface-chrome);
    border: 1px solid var(--accent-action);
    box-shadow: var(--shadow-focus);
  }

  .remove {
    border: 0;
    background: transparent;
    padding: 0;
    cursor: pointer;
    font: inherit;
    color: inherit;
  }

  input {
    flex: 1;
    min-width: 16ch;
    border: 0;
    outline: none;
    background: transparent;
    padding: 0;
    font: var(--machine-md);
    color: var(--text-primary);
    caret-color: var(--caret);
  }

  input::placeholder {
    color: var(--text-faint);
  }

  .help {
    font: var(--machine-sm);
  }

  .login {
    color: var(--text-primary);
  }

  code {
    font: var(--literal);
    color: var(--text-primary);
  }

  .foot {
    font: var(--machine-sm);
    color: var(--text-faint);
  }
</style>
