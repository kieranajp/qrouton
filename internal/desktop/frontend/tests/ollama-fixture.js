import "../src/tokens/typography.css";
import { mount } from "svelte";
import OllamaStatus from "../src/lib/OllamaStatus.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

const params = new URLSearchParams(location.search);
const initial = {
  state: params.get("state") ?? "missing",
  model: "all-minilm:22m-l6-v2-fp16",
  installed: params.get("installed") === "1",
  reason: params.get("reason") ?? "",
};

window.calls = [];
window.emitOllama = (status) =>
  emitWailsEvent("ollama:status", { model: initial.model, ...status });
window.wailsCall = async (name) => {
  window.calls.push(name.split(".").pop());
  if (name.endsWith("Ollama.Check")) return initial;
  return undefined;
};

mount(OllamaStatus, { target: document.querySelector("#fixture") });
