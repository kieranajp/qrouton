import "../src/tokens/scale.css";
import "../src/tokens/typography.css";
import { mount } from "svelte";
import FirstRunOverlay from "../src/lib/firstrun/FirstRunOverlay.svelte";

const refuse = new URLSearchParams(location.search).get("refuse") ?? "";

window.saves = [];
window.wailsCall = async (name, input) => {
  if (name.endsWith("Settings.Load")) return { orgs: [], root: "/sessions" };
  if (name.endsWith("FirstRun.Login")) return "";
  if (name.endsWith("FirstRun.Save")) {
    window.saves.push(input);
    if (refuse) throw new Error(`${refuse}: refused`);
    return { relaunching: true };
  }
  return undefined;
};

mount(FirstRunOverlay, { target: document.querySelector("#fixture") });
