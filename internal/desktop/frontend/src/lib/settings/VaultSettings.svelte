<script>
  import Button from "../core/Button.svelte";
  import TextField from "../forms/TextField.svelte";
  import * as go from "./calls.js";

  /** @type {{profiles: import('./calls.js').VaultProfile[], mappings: Record<string,string>, error?: string}} */
  let { profiles = $bindable([]), mappings = $bindable({}), error = "" } = $props();
  let org = $state("");
  let target = $state("");
  let message = $state("");
  let saving = $state(false);
  let current = $state(/** @type {import('./calls.js').VaultSession | null} */ (null));
  let selected = $state(/** @type {string[]} */ ([]));
  let repositories = $state(/** @type {string[]} */ ([]));
  let workstream = $state("");
  let destination = $state("");
  let publicationDisabled = $state(false);

  function apply(value) {
    current = value;
    selected = [...(value?.selection?.readProfiles ?? [])];
    repositories = [...(value?.selection?.repositories ?? [])];
    workstream = value?.workstream ?? "";
    destination = value?.selection?.destination ?? "";
    publicationDisabled = value?.selection?.publicationDisabled ?? false;
  }
  go.vaultSession().then(apply).catch((err) => (message = String(err)));

  function addProfile() {
    profiles = [...profiles, { id: crypto.randomUUID(), name: "", root: "" }];
  }
  function removeProfile(id) {
    profiles = profiles.filter((profile) => profile.id !== id);
    mappings = Object.fromEntries(Object.entries(mappings).filter(([, profile]) => profile !== id));
  }
  function mapOrg() {
    if (!org.trim() || !target) return;
    mappings = { ...mappings, [org.trim().toLowerCase()]: target };
    org = "";
  }
  async function saveScope() {
    if (!current?.session || saving) return;
    saving = true;
    try {
      apply(await go.saveVaultSession({
        session: current.session,
        workstream,
        selection: { readProfiles: selected, repositories, destination, publicationDisabled },
      }));
      message = "Session vault scope saved.";
    } catch (err) {
      message = String(err);
    } finally {
      saving = false;
    }
  }
</script>

<fieldset>
  <legend>Knowledge vaults</legend>
  <p>Connect an existing Markdown vault. Profiles and organisation mappings are saved with Settings.</p>
  {#each profiles as profile, index (profile.id)}
    <div class="profile">
      <TextField label="Vault name" aria-label="Vault name {index + 1}" bind:value={profile.name} />
      <TextField label="Vault folder" aria-label="Vault folder {index + 1}" bind:value={profile.root} placeholder="/absolute/path/to/vault" />
      <Button variant="ghost" onclick={() => removeProfile(profile.id)}>Remove vault {index + 1}</Button>
    </div>
  {/each}
  <Button variant="secondary" onclick={addProfile}>Add vault</Button>
  {#if profiles.length}
    <div class="mapping">
      <TextField label="GitHub organisation" aria-label="Vault organisation" bind:value={org} />
      <label>Destination vault
        <select aria-label="Destination vault" bind:value={target}>
          <option value="">Choose a vault</option>
          {#each profiles as profile (profile.id)}<option value={profile.id}>{profile.name || "Unnamed vault"}</option>{/each}
        </select>
      </label>
      <Button variant="secondary" onclick={mapOrg}>Map organisation</Button>
    </div>
    {#each Object.entries(mappings) as [owner, id] (owner)}
      <div class="mapping-row">
        <span>{owner} → {profiles.find((p) => p.id === id)?.name ?? id}</span>
        <Button variant="ghost" onclick={() => (mappings = Object.fromEntries(Object.entries(mappings).filter(([key]) => key !== owner)))}>Remove mapping {owner}</Button>
      </div>
    {/each}
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
  {#if current?.session && profiles.length}
    <fieldset>
      <legend>This session</legend>
      <TextField label="Workstream" aria-label="Vault workstream" bind:value={workstream} />
      <p>Save new vault profiles in Settings before selecting them for this session.</p>
      {#if current.status?.scope?.selectionRequired}<p>Select a read profile for this repository-free session.</p>{/if}
      {#if current.repositories.length}
        <p>Repository scope: leave unchecked to use all attached repositories.</p>
        {#each current.repositories as repo (repo)}
          <label class="choice"><input type="checkbox" bind:group={repositories} value={repo} />{repo}</label>
        {/each}
      {/if}
      <p>Read profiles: leave unchecked to follow organisation mappings. Selections may only narrow mapped access.</p>
      {#each profiles as profile (profile.id)}
        <label class="choice"><input type="checkbox" bind:group={selected} value={profile.id} />Read {profile.name || "Unnamed vault"}</label>
      {/each}
      <label>Publication destination
        <select aria-label="Publication destination" bind:value={destination}>
          <option value="">Use organisation routing</option>
          {#each profiles as profile (profile.id)}<option value={profile.id}>{profile.name || "Unnamed vault"}</option>{/each}
        </select>
      </label>
      <label class="choice"><input type="checkbox" bind:checked={publicationDisabled} />Disable publication for this session</label>
      <Button variant="secondary" disabled={saving} onclick={saveScope}>Save session scope</Button>
      {#each current.status?.profiles ?? [] as status (status.profile)}
        <p>{profiles.find((p) => p.id === status.profile)?.name ?? status.profile}: {status.documents} readable documents; {status.invalid} invalid; {status.unsupported} unsupported; {status.conflicts} conflicts. {status.state}.</p>
      {/each}
      {#if current.status?.scope?.unmapped?.length}<p>Unmapped organisations: {current.status.scope.unmapped.join(", ")}</p>{/if}
      <p>Search is unavailable until indexing and relevance calibration are ready. Direct reads require no model.</p>
    </fieldset>
  {/if}
  {#if message}<p role="status">{message}</p>{/if}
</fieldset>

<style>
  fieldset { border: 0; padding: 0; margin: 0; display: grid; gap: 12px; }
  legend { padding: 0 0 10px; font: var(--display-sm); color: var(--text-primary); }
  p { margin: 0; font: var(--machine-sm); color: var(--text-secondary); }
  .profile, .mapping { display: grid; gap: 10px; padding: 12px; border: 1px solid var(--border-subtle); }
  .mapping-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
  label, .mapping-row { font: var(--machine-sm); color: var(--text-secondary); }
  label { display: grid; gap: 6px; }
  .choice { display: flex; align-items: center; gap: 8px; }
  select { background: var(--surface-chrome); color: var(--text-primary); padding: 8px; border: 1px solid var(--border-default); }
  .error { color: var(--state-failed); }
</style>
