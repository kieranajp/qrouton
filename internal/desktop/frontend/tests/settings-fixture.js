import { mount } from "svelte";
import SettingsOverlay from "../src/lib/settings/SettingsOverlay.svelte";

// ?fail=<method> makes that one bridge call reject, which is how a workbench
// that cannot answer is reproduced without a workbench.
const failing = new URLSearchParams(location.search).get("fail") ?? "";
const invalid = new URLSearchParams(location.search).get("invalid") ?? "";
const saves = [];
const scopes = [];
const vaults = new URLSearchParams(location.search).has("vaults");

window.settingsFixture = { saves: () => [...saves], scopes: () => [...scopes] };

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
