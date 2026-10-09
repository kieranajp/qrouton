import "./tokens/index.css";
import { mount } from "svelte";
import Session from "./Session.svelte";
import { startScale } from "./lib/scale.svelte.js";

startScale();
mount(Session, { target: document.body });
