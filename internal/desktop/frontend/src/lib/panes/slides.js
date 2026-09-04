import { Marp } from "@marp-team/marp-core";
import theme from "./slide-theme.css?raw";

// Marp Core turns inlineSVG on for itself, which buries every slide under an
// <svg><foreignObject> and needs the browser script to keep that layer upright
// against Safari. The cards scale themselves, so both are off and each section
// stays a direct child of div.marpit.
const marp = new Marp({ inlineSVG: false, script: false });
marp.themeSet.default = marp.themeSet.add(theme);

/** A deck's slides, stylesheet and per-slide speaker notes.
 * @param {string} markdown
 * @returns {{html: string, css: string, comments: string[][]}} */
export function renderDeck(markdown) {
  return marp.render(markdown ?? "");
}
