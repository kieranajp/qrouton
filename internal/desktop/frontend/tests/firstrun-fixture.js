import "../src/tokens/typography.css";
import { mount } from "svelte";
import FirstRunOverlay from "../src/lib/firstrun/FirstRunOverlay.svelte";

window.saves = [];
window.wailsCall = async (name, input) => {
  if (name.endsWith("Settings.Load")) return { orgs: [], root: "/sessions" };
  if (name.endsWith("FirstRun.Login")) return "";
  if (name.endsWith("FirstRun.Save")) {
    window.saves.push(input);
    return { relaunching: true };
  }
  return undefined;
};

mount(FirstRunOverlay, { target: document.querySelector("#fixture") });
