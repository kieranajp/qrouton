<script>
  import { onMount } from "svelte";
  import Button from "../core/Button.svelte";
  import TextField from "../forms/TextField.svelte";
  import * as go from "./calls.js";

  /** @type {{profiles: import('./calls.js').VaultProfile[]}} */
  let { profiles } = $props();
  let files = $state(/** @type {any[]} */ ([]));
  let manifests = $state(/** @type {any[]} */ ([]));
  let associations = $state({});
  let preview = $state(/** @type {any} */ (null));
  let selected = $state(/** @type {string[]} */ ([]));
  let mappings = $state({ namespaces: [], repositoryAliases: [], linkMappings: [] });
  let jobs = $state(/** @type {any[]} */ ([]));
  let message = $state("");
  let statusError = $state("");
  let busy = $state(false);
  let dirty = $state(false);
  let previous = {};
  let edits = {};
  let alive = true;
  onMount(() => {
    let timer;
    alive = true;
    async function poll() {
      try { const result = await go.vaultImportStatus(); if (alive) { jobs = result.entries ?? []; statusError = ""; } }
      catch (err) { if (alive) statusError = String(err); }
      if (alive) timer = setTimeout(poll, 1500);
    }
    poll();
    return () => { alive = false; clearTimeout(timer); };
  });
  async function act(action) {
    if (busy) return;
    busy = true; message = "";
    try { await action(); } catch (err) { message = String(err); } finally { busy = false; }
  }
  async function choose(kind) {
    const picked = await go.selectVaultImportSources(kind);
    if (kind === "manifest") manifests = [...manifests, ...picked];
    else files = [...files, ...picked];
    if (picked.length) { dirty = true; selected = []; }
  }
  async function reset() {
    await go.clearVaultImportSources(); files = []; manifests = []; associations = {}; preview = null; selected = [];
    mappings = { namespaces: [], repositoryAliases: [], linkMappings: [] }; dirty = false; previous = {}; edits = {};
  }
  async function rebuild() {
    for (const entry of preview?.entries ?? []) {
      const prior = previous[entry.key];
      if (!prior) continue;
      const changes = edits[entry.key] ?? { fields: {} };
      for (const key of Object.keys(entry.document)) {
        if (JSON.stringify(entry.document[key]) !== JSON.stringify(prior.document[key])) changes.fields[key] = JSON.parse(JSON.stringify(entry.document[key]));
      }
      if (entry.body !== prior.body) changes.body = entry.body;
      if (entry.destination !== prior.destination) changes.destination = entry.destination;
      if (Object.keys(changes.fields).length || "body" in changes || "destination" in changes) edits[entry.key] = changes;
    }
    const input = { sources: files.map((file) => file.token), associations, overrides: {} };
    for (const key of ["namespaces", "repositoryAliases", "linkMappings"]) input[key] = Object.fromEntries(mappings[key].filter((row) => row.from && row.to).map((row) => [row.from, row.to]));
    const inferred = await go.previewVaultImport(input);
    for (const entry of inferred.entries) {
      const change = edits[entry.key];
      if (!change) continue;
      const override = {};
      if (Object.keys(change.fields).length) override.document = { ...entry.document, ...change.fields, schemaVersion: 1 };
      if ("body" in change) override.body = change.body;
      if ("destination" in change) override.destination = change.destination;
      input.overrides[entry.key] = override;
    }
    preview = Object.keys(input.overrides).length ? await go.previewVaultImport(input) : inferred;
    previous = JSON.parse(JSON.stringify(Object.fromEntries(preview.entries.map((entry) => [entry.key, entry]))));
    dirty = false; selected = [];
  }
  async function confirm() {
    const report = await go.confirmVaultImport(preview.id, selected);
    jobs = report.entries; message = "Selected documents queued for import.";
  }
  function edit(key) { dirty = true; selected = []; }
  function addRepo(entry) { entry.document.repos = [...(entry.document.repos ?? []), { path: "", role: "editing", revision: "" }]; edit(entry.key); }
  function addEdge(entry) { entry.document.lineage = [...(entry.document.lineage ?? []), { relation: "derived_from", target: "" }]; edit(entry.key); }
</script>

<section aria-label="Legacy import">
  <h3>Import reviewed artifacts</h3>
  <p>Select Markdown and optional session manifests. Originals stay unchanged. Review repairs and canonical content before importing.</p>
  <div class="buttons">
    <Button variant="secondary" disabled={busy || !profiles.length} onclick={() => act(() => choose("markdown"))}>Choose Markdown files</Button>
    <Button variant="secondary" disabled={busy || !profiles.length} onclick={() => act(() => choose("manifest"))}>Choose session manifests</Button>
    {#if files.length || manifests.length}<Button variant="ghost" disabled={busy} onclick={() => act(reset)}>Clear import selection</Button>{/if}
  </div>
  {#if !profiles.length}<p>Save a vault profile to select new sources. Pending imports keep their original destination.</p>{/if}
  {#each files as file (file.token)}
    <label>{file.name} — session manifest
      <select aria-label="Manifest for {file.name}" bind:value={associations[file.token]} onchange={edit}>
        <option value="">No manifest</option>
        {#each manifests as manifest (manifest.token)}<option value={manifest.token}>{manifest.name} ({manifest.session})</option>{/each}
      </select>
    </label>
  {/each}
  {#if files.length}
    <details>
      <summary>Batch mappings</summary>
      {#each [{key:"namespaces",label:"Session namespace"},{key:"repositoryAliases",label:"Repository alias"},{key:"linkMappings",label:"Artifact link"}] as group}
        <h4>{group.label} mappings</h4>
        {#each mappings[group.key] as row, index}
          <div class="row">
            <TextField label="Original {group.label}" aria-label="Original {group.label} {index + 1}" bind:value={row.from} oninput={edit} />
            <TextField label="Resolved {group.label}" aria-label="Resolved {group.label} {index + 1}" bind:value={row.to} oninput={edit} />
          </div>
        {/each}
        <Button variant="ghost" onclick={() => { mappings[group.key] = [...mappings[group.key], { from:"", to:"" }]; edit(); }}>Add {group.label} mapping</Button>
      {/each}
      <p>Link targets must be canonical IDs from the selected documents in the same destination vault.</p>
    </details>
    <Button variant="secondary" disabled={busy} onclick={() => act(rebuild)}>{preview ? "Rebuild import preview" : "Preview import"}</Button>
  {/if}
  {#if preview}
    <p>{preview.accepted} ready; {preview.repairRequired} need repair; {preview.excluded} excluded.</p>
    {#if dirty}<p>Changes need a fresh preview before confirmation.</p>{/if}
    {#each preview.entries as entry (entry.key)}
      <article>
        <label class="choice"><input type="checkbox" value={entry.key} bind:group={selected} disabled={busy || dirty || entry.repairs.length > 0} />Import {entry.name}</label>
        <p>Destination: {profiles.find((profile) => profile.id === entry.destination)?.name ?? "Selection required"}{entry.path ? ` / ${entry.path}` : ""}</p>
        {#each entry.repairs as repair}<p class="repair">{repair.field}: {repair.reason}</p>{/each}
        {#each entry.warnings as warning}<p>{warning.field}: {warning.reason}</p>{/each}
        <details>
          <summary>Edit metadata and content for {entry.name}</summary>
          <div class="editor" oninput={() => edit(entry.key)} onchange={() => edit(entry.key)}>
            {#each [{key:"title",label:"Title"},{key:"author",label:"Author"},{key:"date",label:"Recorded date"},{key:"session",label:"Session namespace"},{key:"id",label:"Artifact ID"},{key:"workstream",label:"Workstream"},{key:"ticket",label:"Ticket URL"},{key:"statusNote",label:"Status note"}] as field}
              <TextField label={field.label} aria-label="{field.label} for {entry.name}" value={entry.document[field.key] ?? ""} oninput={(event) => { entry.document[field.key] = event.target.value; }} />
            {/each}
            <label>Kind<select aria-label="Kind for {entry.name}" bind:value={entry.document.kind}>{#each ["research","spec","plan","note","dead_end"] as kind}<option value={kind}>{kind}</option>{/each}</select></label>
            <label>State<select aria-label="State for {entry.name}" bind:value={entry.document.state}><option value="">Choose state</option>{#each ["active","superseded","abandoned"] as state}<option value={state}>{state}</option>{/each}</select></label>
            <label>Destination<select aria-label="Import destination for {entry.name}" bind:value={entry.destination}><option value="">Use organisation routing</option>{#each profiles as profile}<option value={profile.id}>{profile.name}</option>{/each}</select></label>
            {#each entry.document.repos ?? [] as repo, index}
              <div class="repository">
                <TextField label="Portable repository" aria-label="Repository {index + 1} for {entry.name}" bind:value={repo.path} />
                <label>Role<select aria-label="Repository role {index + 1} for {entry.name}" bind:value={repo.role}><option value="editing">editing</option><option value="reference">reference</option></select></label>
                <TextField label="Historical commit" aria-label="Revision {index + 1} for {entry.name}" value={repo.revision ?? ""} oninput={(event) => { repo.revision = event.target.value || null; }} />
                <Button variant="ghost" onclick={() => {entry.document.repos = entry.document.repos.filter((_, at) => at !== index); edit(entry.key);}}>Remove repository {index + 1}</Button>
              </div>
            {/each}
            <Button variant="ghost" onclick={() => addRepo(entry)}>Add repository for {entry.name}</Button>
            {#each entry.document.lineage ?? [] as edge, index}
              <div class="row"><label>Relation<select bind:value={edge.relation}><option value="derived_from">derived_from</option><option value="supersedes">supersedes</option></select></label><TextField label="Target artifact ID" bind:value={edge.target} /><Button variant="ghost" onclick={() => {entry.document.lineage = entry.document.lineage.filter((_, at) => at !== index); edit(entry.key);}}>Remove lineage {index + 1}</Button></div>
            {/each}
            <Button variant="ghost" onclick={() => addEdge(entry)}>Add lineage for {entry.name}</Button>
            <TextField multiline label="Reviewed Markdown body" aria-label="Body for {entry.name}" bind:value={entry.body} />
          </div>
        </details>
        {#if Object.keys(entry.unsupported ?? {}).length}<details><summary>Retained unsupported fields</summary>{#each Object.entries(entry.unsupported) as [key,value]}<p>{key}</p><pre>{value}</pre>{/each}</details>{/if}
        <details><summary>Canonical preview for {entry.name}</summary><pre>{entry.canonical || "Complete the metadata repairs to generate canonical content."}</pre></details>
      </article>
    {/each}
    <Button variant="secondary" disabled={busy || dirty || !selected.length} onclick={() => act(confirm)}>Import selected valid documents</Button>
  {/if}
  {#if jobs.length}
    <h4>Import queue</h4>
    {#each jobs as job (job.id)}
      <div class="job"><p>{job.artifactId} → {profiles.find((profile) => profile.id === job.profile)?.name ?? job.profile}: {job.state}</p>
        {#if job.error}<p>{job.error}</p>{/if}
        {#if job.state === "paused"}<p>Enable publication in the originating session to resume this import.</p>{/if}
        {#if job.state === "destination_changed"}<p>Confirm that the current folder for this profile is the intended destination.</p><Button variant="secondary" disabled={busy} onclick={() => act(() => go.revalidateVaultImportDestination(job.id))}>Revalidate destination for {job.artifactId}</Button>{/if}
        {#if !["written","conflict","paused","destination_changed","repair_required"].includes(job.state)}<Button variant="ghost" disabled={busy} onclick={() => act(() => go.retryVaultImports(job.profile))}>Retry import {job.artifactId}</Button>{/if}
        {#if job.state === "repair_required" || job.state === "conflict"}<p>Review the source and destination identity, then create a repaired preview. Existing bytes are preserved.</p>{/if}
      </div>
    {/each}
  {/if}
  {#if message}<p role="status">{message}</p>{/if}
  {#if statusError}<p role="status">{statusError}</p>{/if}
</section>

<style>
 section, article, .editor, details, .job { display: grid; gap: 10px; }
 article, .job { border: 1px solid var(--border-subtle); padding: 12px; }
 h3, h4, p { margin: 0; }
 h3 { font: var(--display-sm); color: var(--text-primary); }
 p, label, h4, summary { font: var(--machine-sm); color: var(--text-secondary); }
 label { display: grid; gap: 6px; }
 .choice, .buttons, .row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
 select { padding: 8px; color: var(--text-primary); background: var(--surface-chrome); border: 1px solid var(--border-default); }
 pre { white-space: pre-wrap; overflow-wrap: anywhere; font: var(--machine-sm); }
 .repair { color: var(--state-failed); }
</style>
