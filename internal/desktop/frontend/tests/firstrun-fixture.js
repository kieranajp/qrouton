import "../src/tokens/typography.css";
import { mount } from "svelte";
import FirstRunOverlay from "../src/lib/firstrun/FirstRunOverlay.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

const refuse = new URLSearchParams(location.search).get("refuse") ?? "";

window.saves = [];
window.emitOllama = (status) => emitWailsEvent("ollama:status", status);
window.wailsCall = async (name, input) => {
  if (name.endsWith("Settings.Load")) return { orgs: [], root: "/sessions" };
  if (name.endsWith("FirstRun.Login")) return "";
  if (name.endsWith("Ollama.Check")) return { state: "unreachable", model: "all-minilm" };
  if (name.endsWith("FirstRun.Save")) {
    window.saves.push(input);
    if (refuse) throw new Error(`${refuse}: refused`);
    return { relaunching: true };
  }
  return undefined;
};

mount(FirstRunOverlay, { target: document.querySelector("#fixture") });
