import "../src/tokens/index.css";
import { mount } from "svelte";
import DashboardFixture from "./DashboardFixture.svelte";

const calls = [];

const opened = [];

window.dashboardBridge = { calls: () => [...calls], opened };

window.wailsCall = async (name, ...args) => {
  calls.push({ method: name.split(".").at(-1), args });
};

mount(DashboardFixture, { target: document.querySelector("#fixture") });
