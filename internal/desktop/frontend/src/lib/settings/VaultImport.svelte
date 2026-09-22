<script>
  import Button from "../core/Button.svelte";
  import TextField from "../forms/TextField.svelte";
  import Individual from "./VaultIndividualImport.svelte";
  import * as go from "./calls.js";

  let { profiles } = $props();
  let corpus = $state(/** @type {any} */ (null));
  let preview = $state(/** @type {any} */ (null));
  let detail = $state(/** @type {any} */ (null));
  let target = $state("");
  let author = $state("");
  let batchState = $state("");
  let workstream = $state(true);
  let busy = $state(false);
  let dirty = $state(false);
  let message = $state("");
  let filter = $state("all");
  let selected = $state(/** @type {string[]} */ ([]));
  let scopes = $state(/** @type {any[]} */ ([]));
  let aliases = $state(/** @type {any[]} */ ([]));
  let sessionFilter = $state("");
  let mappings = $state({ namespaces: [], linkMappings: [] });
  let edits = {};
  let baseline = null;
  let editEpoch = 0;

  const duplicateIDs = $derived(new Set((preview?.entries ?? [])
    .filter((entry, at, all) => all.some((other, index) => index !== at && entry.id && other.id === entry.id))
    .map(entry => entry.id)));
  const rows = $derived((preview?.entries ?? []).filter(entry =>
    (!sessionFilter || entry.session === sessionFilter) &&
    (filter === "all" || (filter === "duplicates" ? duplicateIDs.has(entry.id) : entry.disposition === filter))));

  async function act(fn) {
    if (busy) return;
    busy = true;
    message = "";
    try {
      await fn();
    } catch (err) {
      message = String(err);
      selected = [];
    } finally {
      busy = false;
    }
  }

  function change() {
    editEpoch++;
    dirty = true;
    selected = [];
  }

  async function choose() {
    const result = await go.selectVaultImportCorpus();
    if (!result.token) return;
    corpus = result;
    preview = null;
    detail = null;
    edits = {};
    selected = [];
    author = result.author;
    batchState = "";
    target = profiles.find(profile => profile.id === target)?.id ?? profiles[0]?.id ?? "";
    scopes = [];
    mappings = { namespaces: [], linkMappings: [] };
    aliases = Object.entries(result.repositoryAliases ?? {}).map(([from, to]) => ({ from, to }));
    change();
  }

  function remember() {
    if (!detail || !baseline) return;
    const prior = edits[detail.key] ?? { fields: {} };
    for (const key of Object.keys(detail.document)) {
      if (JSON.stringify(detail.document[key]) !== JSON.stringify(baseline.document[key])) {
        prior.fields[key] = structuredClone($state.snapshot(detail.document[key]));
      }
    }
    if (detail.body !== baseline.body) prior.body = detail.body;
    edits[detail.key] = prior;
  }

  async function rebuild() {
    remember();
    detail = null;
    baseline = null;
    const requestedEpoch = editEpoch;
    const input = {
      token: corpus.token,
      targetProfile: target,
      defaults: { author, state: batchState, workstreamFromSession: workstream },
      sessionRepositories: {},
      repositoryAliases: Object.fromEntries(aliases.filter(row => row.from && row.to).map(row => [row.from, row.to])),
      overrides: {},
    };
    for (const row of scopes) {
      for (const session of row.sessions.split(/[\s,]+/).filter(Boolean)) {
        input.sessionRepositories[session] = row.repos.split(/[\s,]+/).filter(Boolean)
          .map(path => ({ path, role: "editing", revision: null }));
      }
    }
    for (const group of ["namespaces", "linkMappings"]) {
      input[group] = Object.fromEntries(mappings[group].filter(row => row.from && row.to).map(row => [row.from, row.to]));
    }
    const fresh = await go.previewVaultCorpus(input);
    for (const [key, edit] of Object.entries(edits)) {
      if (!Object.keys(edit.fields).length && !("body" in edit)) continue;
      const inferred = await go.vaultCorpusEntry(fresh.id, key);
      const override = {};
      if (Object.keys(edit.fields).length) override.document = { ...inferred.document, ...edit.fields, schemaVersion: 1 };
      if ("body" in edit) override.body = edit.body;
      input.overrides[key] = override;
    }
    preview = Object.keys(input.overrides).length ? await go.previewVaultCorpus(input) : fresh;
    dirty = editEpoch !== requestedEpoch;
    if (dirty) selected = [];
    else selectReady();
  }

  function selectReady() {
    selected = (preview?.entries ?? []).filter(entry => entry.disposition === "ready").map(entry => entry.key);
  }

  function toggle(entry, checked) {
    const next = new Set(selected);
    const byKey = new Map(preview.entries.map(row => [row.key, row]));
    function add(key) {
      if (next.has(key)) return;
      const row = byKey.get(key);
      if (!row || row.disposition !== "ready") return;
      next.add(key);
      for (const dependency of row.dependencies ?? []) add(dependency);
    }
    if (checked) add(entry.key);
    else {
      next.delete(entry.key);
      let changed = true;
      while (changed) {
        changed = false;
        for (const key of next) {
          if ((byKey.get(key)?.dependencies ?? []).some(dependency => !next.has(dependency))) {
            next.delete(key);
            changed = true;
          }
        }
      }
    }
    selected = [...next];
  }

  async function inspect(entry) {
    remember();
    detail = await go.vaultCorpusEntry(preview.id, entry.key);
    baseline = structuredClone($state.snapshot(detail));
    const edit = edits[entry.key];
    if (edit) {
      detail.document = { ...detail.document, ...edit.fields };
      if ("body" in edit) detail.body = edit.body;
    }
  }

  async function confirm() {
    const submitted = [...selected];
    await go.confirmVaultImport(preview.id, submitted);
    message = `${submitted.length} documents queued for import.`;
    selected = [];
  }
</script>

<section aria-label="Corpus import">
 <h3>Import an artifact folder</h3>
 <p>Select the folder containing session artifact folders. Originals stay unchanged. Repository scope determines which documents belong in the selected vault.</p>
 <Button variant="secondary" disabled={busy||!profiles.length} onclick={()=>act(choose)}>Choose corpus folder</Button>
 {#if corpus}
  <p>{corpus.name}: {corpus.documents} Markdown files; {corpus.skipped} other or unsafe entries skipped.</p>
  <div class="defaults">
   <label>Destination vault<select bind:value={target} onchange={change}>{#each profiles as profile}<option value={profile.id}>{profile.name}</option>{/each}</select></label>
   <TextField label="Batch author" aria-label="Batch author" bind:value={author} oninput={change}/>
   <label>State for missing legacy metadata<select bind:value={batchState} onchange={change}><option value="">Review and choose</option><option value="active">Active</option><option value="superseded">Superseded</option><option value="abandoned">Abandoned</option></select></label>
  </div>
  <p>Author suggestion comes from local Git configuration. Review it before previewing; existing author metadata is preserved.</p>
  <label class="choice"><input type="checkbox" bind:checked={workstream} onchange={change}/>Use the session namespace as workstream when missing</label>
  <details><summary>Session scope and repository aliases</summary>
   <p>Repository alias suggestions match local mirror identity and configured organisation routing. Review them here. Assign repositories once for sessions with missing scope. Existing declared scope is preserved. Separate session names and repositories with spaces.</p>
   {#if preview}<p>Sessions needing repository repair: {[...new Set(preview.entries.filter(e=>e.repairs?.some(r=>r.field==="repos")).map(e=>e.session))].join(", ")||"None"}</p>{/if}
   {#each scopes as row}<div class="defaults"><TextField label="Session namespaces" bind:value={row.sessions} oninput={change}/><TextField label="Portable repositories" bind:value={row.repos} oninput={change}/></div>{/each}
   {#if preview}<Button variant="ghost" onclick={()=>{scopes=[...scopes,{sessions:[...new Set(preview.entries.filter(e=>e.repairs?.some(r=>r.field==="repos")).map(e=>e.session))].filter(Boolean).join(" "),repos:""}];change();}}>Assign all sessions needing scope</Button>{/if}
   <Button variant="ghost" onclick={()=>{scopes=[...scopes,{sessions:"",repos:""}];change();}}>Assign session repositories</Button>
   {#each aliases as row}<div class="defaults"><TextField label="Recorded repository alias" bind:value={row.from} oninput={change}/><TextField label="Portable repository identity" bind:value={row.to} oninput={change}/></div>{/each}
   <Button variant="ghost" onclick={()=>{aliases=[...aliases,{from:"",to:""}];change();}}>Add repository alias</Button>
   {#each [{key:"namespaces",label:"Session namespace"},{key:"linkMappings",label:"Artifact link"}] as group}<h4>{group.label} mappings</h4>{#each mappings[group.key] as row}<div class="defaults"><TextField label="Original {group.label}" bind:value={row.from} oninput={change}/><TextField label="Resolved {group.label}" bind:value={row.to} oninput={change}/></div>{/each}<Button variant="ghost" onclick={()=>{mappings[group.key]=[...mappings[group.key],{from:"",to:""}];change();}}>Add {group.label} mapping</Button>{/each}
   <p>Artifact links must resolve to selected canonical IDs in this destination vault.</p>
  </details>
  <div class="buttons"><Button variant="secondary" disabled={busy||!author||!batchState||!target} onclick={()=>act(rebuild)}>{preview?"Refresh corpus preview":"Preview corpus"}</Button><Button variant="ghost" disabled={busy} onclick={()=>act(async()=>{await go.clearVaultImportSources();corpus=null;preview=null;detail=null;selected=[];})}>Clear corpus selection</Button></div>
 {/if}
 {#if preview}
  <p class="summary">{preview.total} documents · {preview.ready} ready · {preview.repairRequired} need repair · {preview.excluded} excluded</p>
  <p>Fixed destination: {profiles.find(p=>p.id===preview.targetProfile)?.name??preview.targetProfile}. Historical revisions may be unknown for legacy imports.</p>
  <div class="buttons"><label>Session filter<select bind:value={sessionFilter}><option value="">All sessions</option>{#each [...new Set(preview.entries.map(entry=>entry.session))].sort() as session}<option value={session}>{session||"Unknown session"}</option>{/each}</select></label><label>Show<select bind:value={filter}><option value="all">All documents</option><option value="ready">Ready</option><option value="repair">Needs repair</option><option value="excluded">Excluded</option><option value="duplicates">Duplicate identities ({duplicateIDs.size})</option></select></label><Button variant="ghost" disabled={busy||dirty} onclick={selectReady}>Select all ready</Button><span>{selected.length} selected, including linked dependencies</span></div>
  {#if dirty}<p role="status">Changes need a fresh preview before confirmation.</p>{/if}
  <div class="entries">{#each rows as entry (entry.key)}
   <div class="entry"><label class="choice"><input type="checkbox" checked={selected.includes(entry.key)} disabled={busy||dirty||entry.disposition!=="ready"} onchange={event=>toggle(entry,event.currentTarget.checked)}/><span>{entry.title||entry.name}<small>{entry.session} · {entry.kind} · {entry.disposition}{entry.unknownRevisions?" · revision unknown":""}</small></span></label><Button variant="ghost" disabled={busy} onclick={()=>act(()=>inspect(entry))}>Review {entry.name}</Button>{#if entry.reason}<p>{entry.reason}</p>{/if}</div>
  {/each}</div>
  {#if detail}
   <article aria-label="Artifact exception editor"><h4>{detail.name}</h4><p>{detail.destination} / {detail.path}</p>
    {#each detail.repairs??[] as repair}<p>{repair.field}: {repair.reason}</p>{/each}
    {#each detail.warnings??[] as warning}<p>{warning.field}: {warning.reason}</p>{/each}
    <div class="defaults">{#each ["title","author","date","session","id","workstream","kind","state"] as field}<TextField label={field} value={detail.document[field]??""} oninput={event=>{detail.document[field]=event.target.value;change();}}/>{/each}</div>
    {#each detail.document.repos??[] as repo}<div class="defaults"><TextField label="Repository" bind:value={repo.path} oninput={change}/><TextField label="Historical revision (blank means unknown)" value={repo.revision??""} oninput={event=>{repo.revision=event.target.value||null;change();}}/></div>{/each}
    <TextField multiline label="Reviewed Markdown body" bind:value={detail.body} oninput={change}/>
    <details><summary>Canonical content and inference evidence</summary><pre>{detail.canonical}</pre>{#each Object.entries(detail.provenance??{}) as [field,evidence]}<p>{field}: {evidence.source} — {evidence.detail}</p>{/each}</details>
    <Button variant="ghost" onclick={()=>{remember();detail=null;baseline=null;}}>Close detail</Button>
   </article>
  {/if}
  <Button variant="secondary" disabled={busy||dirty||!selected.length} onclick={()=>act(confirm)}>Import {selected.length} ready documents</Button>
 {/if}
 {#if message}<p role="status">{message}</p>{/if}
</section>
<Individual {profiles}/>
<style>
 section,article,details{display:grid;gap:12px}h3,h4,p{margin:0}h3{font:var(--display-sm)}p,label,span,summary{font:var(--machine-sm)}.defaults,.buttons{display:flex;gap:12px;flex-wrap:wrap;align-items:end}label{display:grid;gap:6px}.choice{display:flex;align-items:center;gap:8px}.entries{max-height:420px;overflow:auto;border:1px solid var(--border-subtle)}.entry{display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap;padding:8px;border-bottom:1px solid var(--border-subtle)}small{display:block;color:var(--text-secondary)}select{padding:8px;background:var(--surface-chrome);color:var(--text-primary);border:1px solid var(--border-default)}article{padding:12px;border:1px solid var(--border-default)}pre{white-space:pre-wrap;overflow-wrap:anywhere}.summary{font-weight:600}
</style>
