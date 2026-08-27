import { mount } from "svelte";
import ChromeFixture from "./ChromeFixture.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

let resolveSnapshot;
window.chromeCalls = [];
window.wailsCall = (name) => {
  window.chromeCalls.push(name);
  return new Promise((resolve) => (resolveSnapshot = resolve));
};
window.resolveChrome = (fields) => resolveSnapshot(fields);
window.emitChrome = (fields) => emitWailsEvent("chrome:update", fields);

mount(ChromeFixture, { target: document.querySelector("#fixture") });
