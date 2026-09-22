import "../src/tokens/index.css";
import { mount } from "svelte";
import SettingsOverlay from "../src/lib/settings/SettingsOverlay.svelte";

// ?fail=<method> makes that one bridge call reject, which is how a workbench
// that cannot answer is reproduced without a workbench.
const failing = new URLSearchParams(location.search).get("fail") ?? "";
const invalid = new URLSearchParams(location.search).get("invalid") ?? "";
const saves = [];
const scopes = [];
const actions = [];
const importCalls = [];
let importJobs = [];
let releaseCorpusPreview;
let releaseCorpusConfirm;
let setupReads = 0;
let download = { state: "", progress: { status: "", total: 0, completed: 0 } };
const dependency = new URLSearchParams(location.search).get("dependency") ?? "model_missing";
const vaults = new URLSearchParams(location.search).has("vaults");

window.settingsFixture = { releaseCorpusConfirm: () => { releaseCorpusConfirm?.(); releaseCorpusConfirm = undefined; }, releaseCorpusPreview: () => { releaseCorpusPreview?.(); releaseCorpusPreview = undefined; }, saves: () => [...saves], scopes: () => [...scopes], actions: () => [...actions], setupReads: () => setupReads, importCalls: () => [...importCalls] };

window.wailsCall = async (name, input, key) => {
  if (failing && name.endsWith("." + failing)) throw new Error("config.json: permission denied");
  if (name.endsWith(".Load"))
    return {
      vaultProfiles: vaults ? [{ id: "shared", name: "Shared", root: "/vault/shared" }, { id: "private", name: "Private", root: "/vault/private" }] : [],
 vaultMappings: vaults ? { acme: "shared" } : {},
 orgs: ["acme"],
      root: "/sessions",
      editor: "",
      launch: "",
      linear: "",
      stickerLabels: {
        star: "Important",
        bookmark: "Read later",
        question: "Needs follow-up",
        exclamation: "Has bugs",
      },
    };
  if (name.endsWith(".VaultImportStatus")) {
    if (new URLSearchParams(location.search).has("pending-import")) return {entries:[{id:"pending",artifactId:"batch/R1",profile:"removed",state:"unavailable",error:"vault profile outside session scope"}]};
    return {entries:importJobs};
  }
  if (name.endsWith(".SelectVaultImportCorpus")) return {token:"corpus",name:"Legacy thoughts",documents:300,skipped:12,sessions:["personal","ambiguous"],author:"Local Author"};
  if (name.endsWith(".PreviewVaultCorpus")) {
    importCalls.push({action:"corpus-preview",input});
    if (new URLSearchParams(location.search).has("delay-corpus")) await new Promise(resolve => { releaseCorpusPreview = resolve; });
    return {id:"corpus-preview",targetProfile:input.targetProfile,total:300,ready:297,repairRequired:2,excluded:1,entries:Array.from({length:300},(_,i)=>({key:`corpus${i}`,id:`personal/R${i}`,name:`R${i}.md`,session:i===298?"ambiguous":"personal",kind:"research",title:`Finding ${i}`,destination:i===299?"private":input.targetProfile,disposition:i===299?"excluded":i>=297?"repair":"ready",dependencies:i===0?["corpus1"]:[],unknownRevisions:true,repairs:i>=297&&i<299?[{field:"repos",reason:"Repository scope needs review"}]:[],warnings:[],reason:i===299?"Different destination vault":""}))};
  }
  if (name.endsWith(".VaultCorpusEntry")) {importCalls.push({action:"detail",key});return {key,name:key+".md",document:{schemaVersion:1,id:"personal/R1",session:"personal",kind:"research",title:"Finding",author:"Local Author",date:"2026-09-22",workstream:"personal",state:"active",repos:[]},body:"Original evidence",canonical:"Canonical evidence",destination:"shared",path:"personal/R1.md",repairs:[],warnings:[],provenance:{author:{source:"batch",detail:"reviewed default"}}};}
  if (name.endsWith(".SelectVaultImportSources")) {
    importCalls.push({action:"pick",kind:input});
    return input === "manifest" ? [{token:"manifest",name:"qrouton.json",kind:"manifest",session:"batch"}] : [{token:"source1",name:"R1.md",kind:"markdown"},{token:"source2",name:"R2.md",kind:"markdown"}];
  }
  if (name.endsWith(".ClearVaultImportSources")) { importCalls.push({action:"clear"}); return; }
  if (name.endsWith(".PreviewVaultImport")) {
    importCalls.push({action:"preview",input:JSON.parse(JSON.stringify(input))});
    const entries = input.sources.map((key) => {
      const associated = Boolean(input.associations[key]);
      const document = {schemaVersion:1,id:"batch/"+(key === "source1" ? "R1" : "R2"),session:associated ? "associated" : "batch",kind:"note",title:key === "source1" ? "First" : "Second",date:"2026-09-21",author:key === "source1" ? "Engineer" : "",state:"active",workstream:associated ? "manifest workstream" : "legacy",repos:[],lineage:[]};
      const override = input.overrides[key];
      const doc = override?.document ?? document;
      const body = override?.body ?? (key === "source1" ? "[next](../batch/R2.md)" : "Original body");
      if (override?.body === "[next](../batch/R2.md)") throw new Error("normalized body must not be reused as source");
      const repairs = doc.author ? [] : [{field:"author",reason:"required field"}];
      return {key,name:key === "source1" ? "R1.md" : "R2.md",document:doc,body,destination:override?.destination || "shared",canonical:repairs.length ? "" : "---\ncanonical: reviewed\n---\n"+body,repairs,warnings:[],unsupported:{}};
    });
    return {id:"preview",entries,accepted:entries.filter((entry) => !entry.repairs.length).length,repairRequired:entries.filter((entry) => entry.repairs.length).length,excluded:0};
  }
  if (name.endsWith(".ConfirmVaultImport")) {
    importCalls.push({action:"confirm"});
    if (new URLSearchParams(location.search).has("stale-import")) throw new Error("source changed since preview");
    if (new URLSearchParams(location.search).has("delay-confirm")) await new Promise(resolve => { releaseCorpusConfirm = resolve; });
    importJobs=[{id:"job",artifactId:"batch/R1",profile:"shared",state:"destination_changed"}];return {entries:importJobs};
  }
  if (name.endsWith(".RevalidateVaultImportDestination")) {importCalls.push({action:"revalidate",id:input});importJobs=importJobs.map((job)=>({...job,state:"pending"}));return;}
  if (name.endsWith(".RetryVaultImports")) {importCalls.push({action:"retry",profile:input});return;}
  if (name.endsWith(".VaultSetup")) {
    setupReads++;
    return { model: "all-minilm:22m-l6-v2-fp16", installed: !new URLSearchParams(location.search).has("not-installed"), download,
      status: { enabled: vaults, profiles: vaults ? ["shared", "private"].map((profile) => ({ profile, documents: 4, invalid: 1, unsupported: 2, conflicts: 3, index: { state: "incomplete", dependency, indexed: 1, pending: 2, excluded: 3, unresolved: 0, competing: 0, chunks: 4, completed: 1, total: 4 } })) : [] } };
  }
  if (name.endsWith(".DownloadVaultModel")) { actions.push("download"); download = { state: "downloading", progress: { status: "downloading", total: 10, completed: 4 } }; return; }
  if (name.endsWith(".CancelVaultDownload")) { actions.push("cancel"); download = { ...download, state: "cancelled" }; return; }
  if (name.endsWith(".RetryVaultIndex")) { actions.push("retry:" + input); return; }
  if (name.endsWith(".VaultSession")) return {
    session: vaults ? "example" : "", workstream: "", selection: {}, repositories: [],
    status: { scope: { selectionRequired: vaults, unmapped: [] }, profiles: [] },
  };
  if (name.endsWith(".SaveVaultSession")) {
    scopes.push(input);
    return { ...input, repositories: [], status: { scope: { selectionRequired: false, unmapped: [] }, profiles: [{ profile: input.selection.readProfiles[0], state: "initializing", documents: 1, invalid: 0, unsupported: 0, conflicts: 0 }] } };
  }
  if (name.endsWith(".Save")) {
    saves.push(input);
    const known = new Set(input.vaultProfiles.map((profile) => profile.id));
    if (Object.values(input.vaultMappings).some((id) => !known.has(id))) throw new Error("vault: invalid vault configuration");
    if (input.vaultProfiles.some((profile) => !profile.name.trim() || !profile.root.startsWith("/"))) throw new Error("vault: invalid vault configuration");
    if (invalid && !input?.stickerLabels?.[invalid]?.trim()) {
      throw new Error(`${invalid}: cannot be empty`);
    }
    return { restartRequired: false };
  }
  return undefined;
};

mount(SettingsOverlay, { target: document.querySelector("#fixture"), props: { onClose: () => {} } });
