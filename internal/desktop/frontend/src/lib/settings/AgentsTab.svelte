<script>
  import { onMount } from "svelte";
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import TextField from "../forms/TextField.svelte";
  import { PROFILE_DEFAULT_LABEL } from "../bridge/generated.js";
  import { Events, call } from "../wails.js";
  import * as go from "./calls.js";
  import { badge, byProfile, followSignIn, pinnedNote, statusKey } from "./profiles.js";
  import SignInBadge from "./SignInBadge.svelte";
  import StatusLine from "./StatusLine.svelte";
  import TabHeader from "./TabHeader.svelte";

  /** @typedef {import("./calls.js").RunnerProfiles} RunnerProfiles */
  /** @typedef {import("./calls.js").ProfileRow} ProfileRow */

  const DEFAULT_PROFILE = Object.freeze({ id: "", label: PROFILE_DEFAULT_LABEL, dir: "", sessions: 0 });

  /** @type {{labelledby?: string}} */
  let { labelledby } = $props();

  let view = $state(/** @type {import("./calls.js").ProfilesView | null} */ (null));
  let loadError = $state("");
  let actionError = $state("");
  let statuses = $state(/** @type {Record<string, import("./calls.js").ProfileStatus>} */ ({}));
  let signingIn = $state(/** @type {Record<string, boolean>} */ ({}));
  let adding = $state(
    /** @type {{runner: string, label: string, dir: string, chosen: boolean, error: string, busy: boolean} | null} */ (null),
  );
  let renaming = $state(/** @type {{runner: string, id: string, label: string, error: string} | null} */ (null));
  let deleting = $state(/** @type {{runner: string, id: string, error: string} | null} */ (null));
  let suggestions = 0;
  /** @type {(() => void)[]} */
  const following = [];

  onMount(() => {
    load();
    return () => following.forEach((stop) => stop());
  });

  async function load() {
    const answer = await call(go.profilesView());
    if (!answer.ok) {
      loadError = message(answer.error);
      return;
    }
    view = answer.value;
    probe();
  }

  async function probe() {
    const answer = await call(go.profileStatus());
    if (answer.ok) statuses = byProfile(answer.value);
  }

  /** @param {any} error */
  const message = (error) => String(error?.message ?? error);

  /** @param {string} event @param {(data: any) => void} listener */
  const on = (event, listener) => Events.On(event, (/** @type {{data: any}} */ incoming) => listener(incoming.data));

  /** @param {RunnerProfiles} runner @param {ProfileRow} profile */
  async function startSignIn(runner, profile) {
    const key = statusKey(runner.id, profile.id);
    if (signingIn[key]) return;
    actionError = "";
    signingIn = { ...signingIn, [key]: true };
    const sign = followSignIn(
      () => call(go.signIn(runner.id, profile.id)),
      on,
      () => {
        signingIn = { ...signingIn, [key]: false };
        probe();
      },
    );
    following.push(sign.stop);
    const answer = await sign.started;
    if (!answer.ok) {
      signingIn = { ...signingIn, [key]: false };
      actionError = message(answer.error);
    }
  }

  /** @param {RunnerProfiles} runner */
  function startAdd(runner) {
    adding = { runner: runner.id, label: "", dir: "", chosen: false, error: "", busy: false };
  }

  async function suggest() {
    if (!adding || adding.chosen) return;
    const asked = ++suggestions;
    const answer = await call(go.suggestProfile(adding.runner, adding.label));
    if (asked !== suggestions || !adding || adding.chosen || !answer.ok) return;
    adding.dir = answer.value.dir;
  }

  // A cancelled picker answers "" with no error, which keeps the folder shown.
  async function chooseFolder() {
    if (!adding) return;
    const answer = await call(go.chooseFolder());
    if (!adding) return;
    if (!answer.ok) {
      adding.error = message(answer.error);
      return;
    }
    if (!answer.value) return;
    adding.dir = answer.value;
    adding.chosen = true;
    adding.error = "";
  }

  async function add() {
    if (!adding || adding.busy) return;
    adding.busy = true;
    const answer = await call(go.addProfile({ runner: adding.runner, label: adding.label, dir: adding.dir }));
    if (!adding) return;
    adding.busy = false;
    if (!answer.ok) {
      adding.error = message(answer.error);
      return;
    }
    view = answer.value;
    adding = null;
    probe();
  }

  /** @param {KeyboardEvent} event */
  function addKey(event) {
    if (event.key === "Escape") {
      event.stopPropagation();
      adding = null;
    } else if (event.key === "Enter") {
      event.preventDefault();
      event.stopPropagation();
      add();
    }
  }

  /** @param {RunnerProfiles} runner @param {ProfileRow} profile */
  function startRename(runner, profile) {
    deleting = null;
    renaming = { runner: runner.id, id: profile.id, label: profile.label, error: "" };
  }

  async function rename() {
    if (!renaming) return;
    const { runner, id, label } = renaming;
    const answer = await call(go.renameProfile(runner, id, label));
    if (!answer.ok) {
      if (renaming) renaming.error = message(answer.error);
      return;
    }
    view = answer.value;
    renaming = null;
  }

  /** @param {KeyboardEvent} event */
  function renameKey(event) {
    if (event.key === "Escape") {
      event.stopPropagation();
      renaming = null;
    } else if (event.key === "Enter") {
      event.preventDefault();
      event.stopPropagation();
      rename();
    }
  }

  /** @param {RunnerProfiles} runner @param {ProfileRow} profile */
  function startDelete(runner, profile) {
    renaming = null;
    deleting = { runner: runner.id, id: profile.id, error: "" };
  }

  async function remove() {
    if (!deleting) return;
    const answer = await call(go.deleteProfile(deleting.runner, deleting.id));
    if (!answer.ok) {
      if (deleting) deleting.error = message(answer.error);
      return;
    }
    view = answer.value;
    deleting = null;
  }

  /** @param {RunnerProfiles} runner @param {ProfileRow} profile */
  const isRenaming = (runner, profile) => renaming?.runner === runner.id && renaming.id === profile.id;

  /** @param {RunnerProfiles} runner @param {ProfileRow} profile */
  const isDeleting = (runner, profile) => deleting?.runner === runner.id && deleting.id === profile.id;
</script>

{#snippet row(/** @type {RunnerProfiles} */ runner, /** @type {ProfileRow} */ profile)}
  {@const key = statusKey(runner.id, profile.id)}
  {@const shown = signingIn[key] ? { text: "Signing in…", tone: "waiting" } : badge(statuses[key])}
  <li aria-label={profile.label}>
    <div class="who">
      {#if renaming && isRenaming(runner, profile)}
        <!-- svelte-ignore a11y_autofocus -->
        <input
          class="rename"
          aria-label="New name for {profile.label}"
          bind:value={renaming.label}
          onkeydown={renameKey}
          autofocus />
      {:else}
        <span class="name">{profile.label}</span>
      {/if}
      <span class="dir" class:inherited={!profile.id}>{profile.id ? profile.dir : "Your usual sign-in"}</span>
      {#if renaming?.error && isRenaming(runner, profile)}<span class="failed">{renaming.error}</span>{/if}
    </div>
    <SignInBadge {shown} />
    <div class="actions">
      {#if isRenaming(runner, profile)}
        <Button variant="cube" size="sm" onclick={rename}>Save name</Button>
        <button type="button" class="link" onclick={() => (renaming = null)}>Cancel</button>
      {:else}
        <Button
          variant="cube"
          size="sm"
          disabled={!view?.sessionShown || signingIn[key]}
          aria-label="Sign in to {runner.label} · {profile.label}"
          onclick={() => startSignIn(runner, profile)}>Sign in</Button>
        {#if profile.id}
          <button type="button" class="link" aria-label="Rename {profile.label}" onclick={() => startRename(runner, profile)}
            >Rename</button>
          <button
            type="button"
            class="link danger"
            aria-label="Delete {profile.label}"
            onclick={() => startDelete(runner, profile)}>Delete</button>
        {/if}
      {/if}
    </div>
  </li>
  {#if deleting && isDeleting(runner, profile)}
    <li class="confirm">
      <StatusLine tone="failed" boxed>
        Delete {profile.label}?
        {#snippet body()}
          <p class="body">
            {pinnedNote(profile)} The folder <code>{profile.dir}</code> stays, with its sign-in and history.
          </p>
          {#if deleting?.error}<p class="body failed">{deleting.error}</p>{/if}
        {/snippet}
        {#snippet actions()}
          <Button variant="destructive" size="sm" onclick={remove}>Delete profile</Button>
          <button type="button" class="link" onclick={() => (deleting = null)}>Keep it</button>
        {/snippet}
      </StatusLine>
    </li>
  {/if}
{/snippet}

<TabHeader title="Agents" id={labelledby}>
  Accounts each coding agent signs in with. A session keeps the profile it started on. Changes here save at once.
</TabHeader>

{#if loadError}
  <StatusLine tone="failed">Profiles could not be read: {loadError}</StatusLine>
{:else if !view}
  <StatusLine>Reading profiles…</StatusLine>
{:else if !view.runners.length}
  <StatusLine>Neither Claude Code nor Codex is installed, so there are no accounts to manage.</StatusLine>
{:else}
  {#if !view.sessionShown}
    <StatusLine>Start or open a session to sign in. The sign-in runs in a tab beside it.</StatusLine>
  {/if}
  {#if actionError}<StatusLine tone="failed">{actionError}</StatusLine>{/if}

  {#each view.runners as runner (runner.id)}
    <section class="runner" aria-labelledby="agents-{runner.id}">
      <CapsLabel id="agents-{runner.id}">{runner.label}</CapsLabel>
      {#if runner.warning}
        <StatusLine tone="waiting" boxed>{runner.warning}</StatusLine>
      {/if}
      <ul class="rows" aria-label="{runner.label} profiles">
        {@render row(runner, DEFAULT_PROFILE)}
        {#each runner.profiles as profile (profile.id)}
          {@render row(runner, profile)}
        {/each}
      </ul>
      {#if adding?.runner === runner.id}
        <div class="add-form" role="group" aria-label="New {runner.label} profile">
          <TextField
            label="Name"
            aria-label="Profile name"
            placeholder="Work"
            bind:value={adding.label}
            oninput={suggest}
            onkeydown={addKey} />
          <div class="folder">
            <CapsLabel>Folder</CapsLabel>
            <div class="folder-row">
              <output class="path" class:empty={!adding.dir} aria-label="Profile folder"
                >{adding.dir || "Follows the name"}</output>
              {#if view?.canChooseFolder}
                <Button variant="secondary" onclick={chooseFolder}>Choose…</Button>
              {/if}
            </div>
            <span class="help" class:failed={adding.error}>
              {adding.error || "Holds this account's sign-in and history. It can't change once the profile is added."}
            </span>
          </div>
          <div class="add-actions">
            <Button variant="cube" disabled={adding.busy} onclick={add}>Add</Button>
            <button type="button" class="link" onclick={() => (adding = null)}>Cancel</button>
          </div>
        </div>
      {:else}
        <div>
          <Button variant="cube" icon="new" aria-label="Add {runner.label} profile" onclick={() => startAdd(runner)}
            >Add profile</Button>
        </div>
      {/if}
    </section>
  {/each}
{/if}

<style>
  .runner {
    display: flex;
    flex-direction: column;
    gap: calc(10px * var(--ui-scale));
  }

  .rows {
    display: flex;
    flex-direction: column;
    margin: 0;
    padding: 0;
    list-style: none;
    border-top: 1px solid var(--border-subtle);
  }

  li {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) calc(230px * var(--ui-scale));
    align-items: center;
    gap: calc(14px * var(--ui-scale));
    padding: calc(9px * var(--ui-scale)) 0;
    border-bottom: 1px solid var(--border-subtle);
  }

  li.confirm {
    display: block;
  }

  .who {
    display: flex;
    flex-direction: column;
    gap: calc(3px * var(--ui-scale));
    min-width: 0;
  }

  .name {
    font: var(--machine-sm);
    color: var(--text-primary);
  }

  .rename {
    border: 0;
    outline: none;
    padding: 0;
    background: var(--surface-chrome);
    box-shadow: inset 0 calc(-1px * var(--ui-scale)) 0 var(--accent-action);
    font: var(--machine-sm);
    color: var(--text-primary);
    caret-color: var(--caret);
  }

  .dir {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font: var(--literal);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-muted);
  }

  .dir.inherited {
    font: var(--machine-xs);
    color: var(--text-faint);
  }

  .actions,
  .add-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: calc(12px * var(--ui-scale));
  }

  .add-actions {
    justify-content: flex-start;
  }

  .link {
    border: 0;
    background: none;
    padding: 0;
    font: var(--machine-sm);
    color: var(--text-faint);
    cursor: pointer;
  }

  .link:hover,
  .link:focus-visible {
    color: var(--text-primary);
    outline: none;
  }

  .link.danger:hover,
  .link.danger:focus-visible {
    color: var(--state-failed);
  }

  .add-form {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1.5fr);
    gap: calc(14px * var(--ui-scale)) calc(18px * var(--ui-scale));
    align-items: start;
  }

  .add-actions {
    grid-column: 1 / -1;
  }

  .folder {
    display: flex;
    flex-direction: column;
    gap: calc(7px * var(--ui-scale));
    min-width: 0;
  }

  .folder-row {
    display: flex;
    gap: calc(8px * var(--ui-scale));
  }

  .path {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    border: 1px solid var(--border-subtle);
    padding: calc(9px * var(--ui-scale)) calc(12px * var(--ui-scale));
    font: var(--literal);
    color: var(--text-secondary);
  }

  .path.empty {
    font: var(--machine-md);
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

  .body {
    margin: 0;
    font: var(--machine-sm);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-muted);
    text-wrap: pretty;
  }

  code {
    font: var(--literal);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-primary);
  }

  .failed {
    font: var(--machine-xs);
    color: var(--state-failed);
  }
</style>
