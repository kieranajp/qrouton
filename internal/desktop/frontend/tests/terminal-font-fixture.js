import "../src/tokens/index.css";
import { fontsReady } from "../src/lib/xterm.js";

const faces = (family) => [...document.fonts].filter((face) => face.family.includes(family));

window.terminalFaces = fontsReady().then(() => {
  const patched = faces("JetBrainsMono Nerd Font Mono");
  return patched.length === 2 && patched.every((face) => face.status === "loaded");
});
