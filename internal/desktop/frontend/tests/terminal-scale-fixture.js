import "../src/tokens/index.css";
import { mount } from "svelte";
import { UI_SCALE_EVENT } from "../src/lib/bridge/generated.js";
import { startScale } from "../src/lib/scale.svelte.js";
import { terminalAt } from "../src/lib/xterm.js";
import Terminal from "../src/lib/session/Terminal.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

startScale();

const pty = { start: "pty.Start", write: "pty.Write", resize: "pty.Resize", data: "pty:data:", exit: "pty:exit:" };
const calls = [];
window.wailsCall = async (name, ...args) => {
  calls.push([name, ...args]);
};

window.terminalScale = {
  calls: () => calls.map((call) => [...call]),
  announce: (percent) => emitWailsEvent(UI_SCALE_EVENT, percent),
  fontSize: () => terminalAt(document.querySelector(".xterm"))?.options.fontSize,
};

mount(Terminal, { target: document.querySelector("#fixture"), props: { id: "t1", pty, active: true } });
