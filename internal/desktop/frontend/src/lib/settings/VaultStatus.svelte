<script>
  import { onMount } from "svelte";
  import Button from "../core/Button.svelte";
  import * as go from "./calls.js";

  /** @type {{profiles: import('./calls.js').VaultProfile[]}} */
  let { profiles } = $props();
  let setup = $state(/** @type {import('./calls.js').VaultSetup | null} */ (null));
  let message = $state("");
  let busy = $state(false);
  let alive = true;
  async function refresh() {
    try {
      const value = await go.vaultSetup();
      if (alive) setup = value;
    } catch (err) {
      if (alive) message = String(err);
    }
  }
  onMount(() => {
    let timer;
    alive = true;
    async function poll() {
      await refresh();
      if (alive) timer = setTimeout(poll, 1000);
    }
    poll();
    return () => { alive = false; clearTimeout(timer); };
  });
  async function act(action) {
    if (busy) return;
    busy = true;
    message = "";
    try { await action(); await refresh(); }
    catch (err) { message = String(err); }
    finally { busy = false; }
  }
  const missingService = $derived(setup?.status.profiles.some((profile) => profile.index?.dependency === "service_unavailable"));
  const missingModel = $derived(setup?.status.profiles.some((profile) => profile.index?.dependency === "model_missing"));
  const incompatible = $derived(setup?.status.profiles.some((profile) => profile.index?.dependency === "incompatible"));
</script>

{#if setup?.status.enabled}
  <section aria-label="Vault indexing">
    <h3>Local indexing</h3>
    {#if missingService}
      {#if setup.installed}
        <p>Ollama is stopped or unreachable. Open Ollama, then retry indexing.</p>
      {:else}
        <p>Ollama was not found. Install it from <a href="https://ollama.com/download" target="_blank" rel="noreferrer">ollama.com/download</a>, open it, then retry indexing.</p>
      {/if}
    {/if}
    {#if missingModel}<p>The required model {setup.model} is missing. Download it explicitly to enable local indexing.</p>{/if}
    {#if incompatible}<p>The embedding model is incompatible. Check the exact model {setup.model} in Ollama, then retry indexing.</p>{/if}
    {#if setup.download.state === "downloading"}
      <p role="status">Downloading {setup.model}: {setup.download.progress.status || "Starting"}</p>
      {#if setup.download.progress.total > 0}
        <progress aria-label="Model download" max={setup.download.progress.total} value={setup.download.progress.completed}></progress>
        <p>{setup.download.progress.completed} / {setup.download.progress.total} bytes</p>
      {/if}
      <Button variant="secondary" disabled={busy} onclick={() => act(go.cancelVaultDownload)}>Cancel download</Button>
    {:else}
      {#if missingModel}<Button variant="secondary" disabled={busy} onclick={() => act(go.downloadVaultModel)}>Download required model</Button>{/if}
      {#if setup.download.state === "cancelled"}<p role="status">Model download cancelled.</p>{/if}
      {#if setup.download.state === "complete"}<p role="status">Model downloaded. Indexing will retry.</p>{/if}
      {#if setup.download.error}<p role="status">{setup.download.error}</p>{/if}
    {/if}
    {#each setup.status.profiles as profile (profile.profile)}
      <div class="index-profile">
        <h4>{profiles.find((item) => item.id === profile.profile)?.name ?? profile.profile}</h4>
        <p>{profile.documents} readable; {profile.invalid} invalid; {profile.unsupported} unsupported; {profile.conflicts} conflicts.</p>
        {#if profile.index}
          <p>{profile.index.state}: {profile.index.indexed} indexed; {profile.index.pending} pending; {profile.index.excluded} excluded.</p>
          {#if profile.index.total > 0 && profile.index.completed < profile.index.total}
            <progress aria-label="Indexing {profile.profile}" max={profile.index.total} value={profile.index.completed}></progress>
            <p>{profile.index.completed} / {profile.index.total} chunks embedded</p>
          {/if}
          {#if profile.index.error}<p>{profile.index.error}</p>{/if}
          {#if profile.index.unresolved || profile.index.competing}<p>{profile.index.unresolved} unresolved lineage links; {profile.index.competing} competing successors.</p>{/if}
          <Button variant="ghost" disabled={busy} onclick={() => act(() => go.retryVaultIndex(profile.profile))}>Retry indexing {profiles.find((item) => item.id === profile.profile)?.name ?? profile.profile}</Button>
        {/if}
      </div>
    {/each}
    <p>Search remains uncalibrated. Direct document reads work while indexing or model setup is unavailable.</p>
    <p>Exclude each vault’s .qrouton/index folder from sync where your sync service supports exclusions. This disposable cache belongs to this installation.</p>
  </section>
{/if}
{#if message}<p role="status">{message}</p>{/if}

<style>
  section { display: grid; gap: 12px; }
  h3, h4, p { margin: 0; }
  h3 { font: var(--display-sm); color: var(--text-primary); }
  h4 { font: var(--machine-sm); color: var(--text-primary); }
  p { font: var(--machine-sm); color: var(--text-secondary); }
  a { color: var(--text-primary); }
  .index-profile { display: grid; gap: 8px; padding: 12px; border: 1px solid var(--border-subtle); }
  progress { width: 100%; accent-color: var(--text-primary); }
</style>
