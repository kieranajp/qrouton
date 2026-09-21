import { mount } from "svelte";
import SettingsOverlay from "../src/lib/settings/SettingsOverlay.svelte";

// ?fail=<method> makes that one bridge call reject, which is how a workbench
// that cannot answer is reproduced without a workbench.
const failing = new URLSearchParams(location.search).get("fail") ?? "";
const invalid = new URLSearchParams(location.search).get("invalid") ?? "";
const saves = [];
const scopes = [];
const actions = [];
let setupReads = 0;
let download = { state: "", progress: { status: "", total: 0, completed: 0 } };
const dependency = new URLSearchParams(location.search).get("dependency") ?? "model_missing";
const vaults = new URLSearchParams(location.search).has("vaults");

window.settingsFixture = { saves: () => [...saves], scopes: () => [...scopes], actions: () => [...actions], setupReads: () => setupReads };

window.wailsCall = async (name, input) => {
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
