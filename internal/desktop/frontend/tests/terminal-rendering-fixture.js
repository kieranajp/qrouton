import "../src/tokens/index.css";
import { encode, fontsReady, mount, paint } from "../src/lib/xterm.js";

if (new URLSearchParams(location.search).has("fallback")) {
  const original = HTMLCanvasElement.prototype.getContext;
  HTMLCanvasElement.prototype.getContext = function (kind, ...args) {
    return kind.startsWith("webgl") ? null : original.call(this, kind, ...args);
  };
}
await fontsReady();
const terminals = [];
window.fixture = {
  terminals,
  create() {
    const host = document.createElement("div");
    host.style.cssText = "width:800px;height:360px";
    document.body.append(host);
    const replies = [];
    const mounted = mount(host, { write: (data) => replies.push(data) });
    const entry = { ...mounted, host, replies, completed: 0 };
    mounted.term.parser.registerOscHandler(777, () => { entry.completed++; return true; });
    terminals.push(entry);
    return terminals.length - 1;
  },
  send(id, text, replay = false, structured = true) {
    paint(terminals[id].term, structured ? { encoded: encode(text), replay } : encode(text));
  },
  bytes(id, bytes) {
    paint(terminals[id].term, btoa(String.fromCharCode(...bytes)));
  },
  done(id) { this.send(id, "\x1b]777;done\x07"); },
  hold(id) {
    const entry = terminals[id];
    entry.term.parser.registerOscHandler(778, () => new Promise((resolve) => {
      entry.held = true;
      entry.release = () => resolve(true);
    }));
  },
  lines(id) {
    const buffer = terminals[id].term.buffer.active;
    return Array.from({ length: buffer.length }, (_, row) => buffer.getLine(row).translateToString(true));
  },
};
