import "../src/tokens/index.css";
import { createTerminalPainter, encode, fontsReady, mount } from "../src/lib/xterm.js";

const fallback = new URLSearchParams(location.search).has("fallback");
const original = HTMLCanvasElement.prototype.getContext;
let contexts = 0;
HTMLCanvasElement.prototype.getContext = function (kind, ...args) {
  if (fallback && kind.startsWith("webgl")) return null;
  const context = original.call(this, kind, ...args);
  if (kind === "webgl2" && context) contexts++;
  return context;
};
await fontsReady();
const terminals = [];
window.fixture = {
  terminals,
  get contexts() { return contexts; },
  create() {
    const host = document.createElement("div");
    host.style.cssText = "width:800px;height:360px";
    document.body.append(host);
    const replies = [];
    const mounted = mount(host, { write: (data) => replies.push(data) });
    const painter = createTerminalPainter(mounted.term);
    const entry = {
      ...mounted,
      host,
      replies,
      completed: 0,
      painter,
      // A terminal outlives its painter nowhere, so the fixture tears both down
      // together the way the session surface does.
      dispose() {
        painter.dispose();
        mounted.dispose();
      },
    };
    mounted.term.parser.registerOscHandler(777, () => { entry.completed++; return true; });
    terminals.push(entry);
    mounted.show();
    return terminals.length - 1;
  },
  send(id, text, replay = false) {
    terminals[id].painter.paint({ encoded: encode(text), replay });
  },
  bytes(id, bytes) {
    terminals[id].painter.paint({ encoded: btoa(String.fromCharCode(...bytes)) });
  },
  done(id) { this.send(id, "\x1b]777;done\x07"); },
  hold(id) {
    const entry = terminals[id];
    entry.term.parser.registerOscHandler(778, () => new Promise((resolve) => {
      entry.held = true;
      entry.release = () => resolve(true);
    }));
  },
  show(id) {
    terminals[id].host.style.display = "";
    terminals[id].show();
  },
  hide(id) {
    terminals[id].host.style.display = "none";
    terminals[id].hide();
  },
  renderer(id) {
    const renderer = terminals[id].term._core._renderService._renderer.value;
    return { webgl: !!renderer._gl, lost: renderer._gl?.isContextLost() };
  },
  lose(id) {
    terminals[id].term._core._renderService._renderer.value._gl.getExtension("WEBGL_lose_context").loseContext();
  },
  rows(id) { return terminals[id].host.querySelector(".xterm-rows")?.textContent ?? ""; },
  lines(id) {
    const buffer = terminals[id].term.buffer.active;
    return Array.from({ length: buffer.length }, (_, row) => buffer.getLine(row).translateToString(true));
  },
};
