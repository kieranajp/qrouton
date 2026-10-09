import { UI_SCALE_EVENT } from "./bridge/generated.js";
import { isNarrow } from "./scale.js";
import { adjustUIScale } from "./settings/calls.js";
import { scaleAction } from "./shortcuts.js";
import { call, Events } from "./wails.js";

const linux = navigator.userAgent.includes("Linux");

/** @type {Set<(percent: number) => void>} */
const listeners = new Set();
let percent = $state(100);

/** The UI scale as a whole percent. */
export const uiScale = () => percent;

/** @param {(percent: number) => void} listener */
export function onScale(listener) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/** The workbench saves the step and announces it back; the page applies only the announcement. */
function onKey(event) {
  const action = scaleAction(event, linux);
  if (!action) return;
  event.preventDefault();
  call(adjustUIScale(action));
}

function markNarrow() {
  document.documentElement.toggleAttribute("data-narrow", isNarrow(window.innerWidth, percent));
}

/** Reads the first-paint scale, then follows the workbench's announcements. */
export function startScale() {
  const painted = parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--ui-scale"));
  if (Number.isFinite(painted)) percent = Math.round(painted * 100);
  markNarrow();
  window.addEventListener("resize", markNarrow);
  window.addEventListener("keydown", onKey);
  const stop = Events.On(UI_SCALE_EVENT, (event) => applyScale(event.data));
  return () => {
    stop();
    window.removeEventListener("resize", markNarrow);
    window.removeEventListener("keydown", onKey);
  };
}

/** @param {unknown} next */
function applyScale(next) {
  if (typeof next !== "number" || !Number.isFinite(next) || next <= 0 || next === percent) return;
  percent = next;
  document.documentElement.style.setProperty("--ui-scale", String(next / 100));
  markNarrow();
  for (const listener of listeners) listener(next);
}
