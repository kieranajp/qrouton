import "../src/tokens/index.css";
import { mount } from "svelte";
import SessionRailFixture from "./SessionRailFixture.svelte";

window.wailsCall = async () => undefined;
mount(SessionRailFixture, { target: document.querySelector("#fixture") });

