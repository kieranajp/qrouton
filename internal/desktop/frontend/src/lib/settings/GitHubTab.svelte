<script>
  import { onMount } from "svelte";
  import { GITHUB_CREDENTIALS, GITHUB_TOKEN_CHECKS, ORGS_ENV_VAR } from "../bridge/generated.js";
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import { openURL } from "../wails.js";
  import CommandRow from "./CommandRow.svelte";
  import { pinnedNote } from "./pinned.js";
  import StatusLine from "./StatusLine.svelte";
  import TabHeader from "./TabHeader.svelte";

  const CLI_URL = "https://cli.github.com";

  /** @type {{panel: ReturnType<typeof import("./settings.svelte.js").settings>, labelledby?: string}} */
  let { panel, labelledby } = $props();

  let form = $derived(panel.form);
  let help = $derived(panel.fields.orgs ?? (form.pinned.orgs ? pinnedNote(ORGS_ENV_VAR) : ""));
  let gh = $derived(panel.github);
  let cli = $derived(gh?.state === GITHUB_CREDENTIALS.CLI);
  let source = $derived(cli ? "the GitHub CLI" : gh?.variable);

  onMount(() => {
    if (!panel.github) panel.checkGitHub();
  });
</script>

<TabHeader title="GitHub" id={labelledby}>
  Orgs and personal accounts whose repositories appear in the picker.
</TabHeader>

{#if gh?.check === GITHUB_TOKEN_CHECKS.VALID}
  <StatusLine tone="success">
    {#if gh.login}Signed in as <span class="login">{gh.login}</span> via {source}{:else}Signed in via {source}{/if}
  </StatusLine>
{:else if gh?.check === GITHUB_TOKEN_CHECKS.REJECTED}
  <StatusLine tone="failed" boxed>
    GitHub rejected the token from {source}
    {#snippet body()}
      {#if cli}
        <p class="body">gh is signed in, but GitHub no longer accepts its token. Refresh the sign-in, or sign in again.</p>
        <div class="commands">
          <CommandRow command="gh auth refresh" />
          <CommandRow command="gh auth login" />
        </div>
      {:else}
        <p class="body">
          Set {gh.variable} to a working token and start qrouton again, or sign in with gh and qrouton uses that instead.
        </p>
        <CommandRow command="gh auth login" />
      {/if}
    {/snippet}
    {#snippet actions()}
      <Button variant="cube" disabled={panel.checkingGitHub} onclick={panel.checkGitHub}>Check again</Button>
    {/snippet}
  </StatusLine>
{:else if gh?.check === GITHUB_TOKEN_CHECKS.UNREACHABLE}
  <StatusLine tone="waiting" boxed detail="Found a token from {source}, but GitHub didn't answer">
    Couldn't reach GitHub to check the sign-in
    {#snippet actions()}
      <Button variant="cube" disabled={panel.checkingGitHub} onclick={panel.checkGitHub}>Check again</Button>
    {/snippet}
  </StatusLine>
{:else if gh?.state === GITHUB_CREDENTIALS.MISSING}
  <StatusLine tone="faint" boxed>
    The GitHub CLI isn't installed
    {#snippet body()}
      <p class="body">
        qrouton lists your repos with the account gh is signed in to. Until it's installed, the repo picker can't
        search these orgs. You can still add them now.
      </p>
      <div class="commands">
        <CommandRow command="brew install gh" />
        <CommandRow command="gh auth login" />
      </div>
    {/snippet}
    {#snippet actions()}
      <Button variant="cube" disabled={panel.checkingGitHub} onclick={panel.checkGitHub}>Check again</Button>
      <a href={CLI_URL} onclick={(event) => (event.preventDefault(), openURL(CLI_URL))}>cli.github.com</a>
    {/snippet}
  </StatusLine>
{:else if gh?.state === GITHUB_CREDENTIALS.SIGNED_OUT}
  <StatusLine tone="waiting" boxed>
    The GitHub CLI isn't signed in
    {#snippet body()}
      <p class="body">gh is installed, but has no account. Sign in once in a terminal and qrouton uses the same account.</p>
      <CommandRow command="gh auth login" />
    {/snippet}
    {#snippet actions()}
      <Button variant="cube" disabled={panel.checkingGitHub} onclick={panel.checkGitHub}>Check again</Button>
    {/snippet}
  </StatusLine>
{:else}
  <StatusLine>Checking the GitHub CLI…</StatusLine>
{/if}

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
  .login {
    color: var(--text-primary);
  }

  .body {
    margin: 0;
    font: var(--machine-sm);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-muted);
    text-wrap: pretty;
  }

  .commands {
    display: flex;
    flex-direction: column;
  }

  a {
    font: var(--machine-xs);
    color: var(--accent-action);
    text-decoration: none;
  }

  a:hover {
    color: var(--ctp-lavender);
  }

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
