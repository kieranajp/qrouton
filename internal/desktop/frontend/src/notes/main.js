import "../tokens/index.css";
import { mount } from "svelte";
import Notes from "./Notes.svelte";
import { startScale } from "../lib/scale.svelte.js";

startScale();
mount(Notes, { target: document.body });
