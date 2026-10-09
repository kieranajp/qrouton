import { mount } from "svelte";
import { emitWailsEvent } from "./wails-runtime.js";
import { UI_SCALE_EVENT } from "../src/lib/bridge/generated.js";
import SettingsOverlay from "../src/lib/settings/SettingsOverlay.svelte";

// ?fail=<method> makes that one bridge call reject, which is how a workbench
// that cannot answer is reproduced without a workbench.
const failing = new URLSearchParams(location.search).get("fail") ?? "";
const invalid = new URLSearchParams(location.search).get("invalid") ?? "";
const saves = [];

window.settingsFixture = {
  saves: () => [...saves],
  announceScale: (percent) => emitWailsEvent(UI_SCALE_EVENT, percent),
};

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
      uiScale: 130,
      uiScaleSteps: [80, 90, 100, 110, 120, 130, 140, 150],
    };
  if (name.endsWith(".Save")) {
    saves.push(input);
    if (invalid && !input?.stickerLabels?.[invalid]?.trim()) {
      throw new Error(`${invalid}: cannot be empty`);
    }
    return { restartRequired: false };
  }
  return undefined;
};

mount(SettingsOverlay, { target: document.querySelector("#fixture"), props: { onClose: () => {} } });
