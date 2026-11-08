import "./tokens/index.css";
import { mount } from "svelte";
import Session from "./Session.svelte";
import { startScale } from "./lib/scale.svelte.js";
import { markPlatform } from "./lib/platform.js";

markPlatform();
startScale();
mount(Session, { target: document.body });
