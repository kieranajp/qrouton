import { mount } from "svelte";
import SettingsOverlay from "../src/lib/settings/SettingsOverlay.svelte";

// ?fail=<method> makes that one bridge call reject, which is how a workbench
// that cannot answer is reproduced without a workbench.
const failing = new URLSearchParams(location.search).get("fail") ?? "";
const invalid = new URLSearchParams(location.search).get("invalid") ?? "";
const saves = [];

window.settingsFixture = { saves: () => [...saves] };

window.wailsCall = async (name, input) => {
  if (failing && name.endsWith("." + failing)) throw new Error("config.json: permission denied");
  if (name.endsWith(".Load"))
    return {
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
      chime: true,
      thoughts: {
        default: "",
        derived: "/sessions/thoughts",
        roots: [
          { id: "work", path: "/sync/work", orgs: "acme" },
          { id: "club", path: "/sync/club", orgs: "club, club-labs" },
        ],
      },
    };
  if (name.endsWith(".Save")) {
    saves.push(input);
    if (invalid === "thoughts") throw new Error("thoughts: an org maps to two thoughts roots");
    if (invalid && !input?.stickerLabels?.[invalid]?.trim()) {
      throw new Error(`${invalid}: cannot be empty`);
    }
    return { restartRequired: false };
  }
  return undefined;
};

mount(SettingsOverlay, { target: document.querySelector("#fixture"), props: { onClose: () => {} } });
