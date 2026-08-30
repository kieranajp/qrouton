import { mount } from "svelte";
import ViewportEpochFixture from "./ViewportEpochFixture.svelte";

window.reports = [];
window.wailsCall = async (name, id, payload) => {
  if (name.endsWith(".RenderDiagrams")) return [];
  if (name.endsWith(".ReportViewport")) window.reports.push(payload);
  return undefined;
};

mount(ViewportEpochFixture, { target: document.querySelector("#fixture") });
