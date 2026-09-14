import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import Notes from "../src/notes/Notes.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

export const RETAINED = {
  index: 2,
  total: 7,
  title: "Third",
  html: "<p>The note the workbench was holding.</p>",
};

const params = new URLSearchParams(location.search);

window.wailsCall = async (name) => {
  if (!name.endsWith(".Note")) return undefined;
  return params.get("blank") ? { index: 0, total: 0, title: "", html: "" } : RETAINED;
};

window.pushNote = (note) => emitWailsEvent("presenter:notes", note);

window.shown = () => ({
  counter: document.querySelector(".counter")?.textContent.trim() ?? "",
  title: document.querySelector(".title").textContent.trim(),
  body: document.querySelector(".body").innerHTML.trim(),
  lists: document.querySelectorAll(".body ul li").length,
  links: [...document.querySelectorAll(".body a")].map((a) => a.getAttribute("href")),
});

mount(Notes, { target: document.body });
