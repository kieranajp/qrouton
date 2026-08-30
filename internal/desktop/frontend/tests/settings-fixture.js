import { mount } from "svelte";
import SettingsOverlay from "../src/lib/settings/SettingsOverlay.svelte";

// ?fail=<method> makes that one bridge call reject, which is how a workbench
// that cannot answer is reproduced without a workbench.
const failing = new URLSearchParams(location.search).get("fail") ?? "";

window.wailsCall = async (name) => {
  if (failing && name.endsWith("." + failing)) throw new Error("config.json: permission denied");
  if (name.endsWith(".Load")) return { orgs: ["acme"], root: "/sessions", editor: "", launch: "", linear: "" };
  return undefined;
};

mount(SettingsOverlay, { target: document.querySelector("#fixture"), props: { onClose: () => {} } });
