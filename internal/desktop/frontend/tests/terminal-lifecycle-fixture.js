import "../src/tokens/index.css";
import { mount, unmount } from "svelte";
import { encode, terminalAt } from "../src/lib/xterm.js";
import Terminal from "../src/lib/session/Terminal.svelte";
import { tabPTY as pty } from "../src/lib/session/services.js";
import { emitWailsEvent } from "./wails-runtime.js";

const params = new URLSearchParams(location.search);
const calls = [];
let exitCode = params.has("completed") ? Number(params.get("completed")) : undefined;
let retained = typeof exitCode === "number" ? "final marker" : "";
const replay = () => emitWailsEvent(pty.data + "t1", { encoded: encode(retained), replay: true, exit_code: exitCode });

window.wailsCall = async (name, ...args) => {
  calls.push([name, ...args]);
  if (name === pty.start) {
    if (params.has("start-error")) throw new Error("fixture start failed");
    replay();
  }
};

const attach = () => mount(Terminal, { target: document.querySelector("#fixture"), props: { id: "t1", pty, active: true } });
let component = attach();

window.terminalLifecycle = {
  calls: () => calls.map((call) => [...call]),
  starts: () => calls.filter(([name]) => name === pty.start),
  lines() {
    const term = terminalAt(document.querySelector(".xterm"));
    if (!term) return [];
    const buffer = term.buffer.active;
    return Array.from({ length: buffer.length }, (_, row) => buffer.getLine(row).translateToString(true)).filter(Boolean);
  },
  finish(code) {
    retained = "final marker";
    exitCode = code;
    emitWailsEvent(pty.data + "t1", { encoded: encode(retained) });
    emitWailsEvent(pty.exit + "t1", code);
  },
  repeatExit: () => emitWailsEvent(pty.exit + "t1", exitCode),
  replay,
  flush() {
    const term = terminalAt(document.querySelector(".xterm"));
    return new Promise((resolve) => {
      const guard = term.parser.registerOscHandler(777, () => {
        guard.dispose();
        resolve();
        return true;
      });
      emitWailsEvent(pty.data + "t1", { encoded: encode("\x1b]777;done\x07") });
    });
  },
  async remount() {
    await unmount(component);
    component = attach();
  },
};
