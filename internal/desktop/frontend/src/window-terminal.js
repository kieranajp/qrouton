import "./tokens/index.css";
import "./window-terminal.css";
import { Call, Events } from "./lib/wails.js";
import { decode, encode, mount, watchSize } from "./lib/xterm.js";

const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

const id = new URLSearchParams(location.search).get("id");
const host = document.getElementById("term");
const write = (text) => Call.ByName(WINDOWS_SERVICE + ".Write", id, encode(text));
const { term, refit } = mount(host, { write });

term.onBinary((data) => Call.ByName(WINDOWS_SERVICE + ".Write", id, btoa(data)));

// Emit(name, one) delivers the value itself; indexing it would take the first
// character of the base64 string and fail silently in here.
Events.On("window:data:" + id, (event) => term.write(decode(event.data)));
Events.On("window:exit:" + id, (event) => {
  term.write("\r\n\x1b[2m[exited with status " + event.data + "]\x1b[0m\r\n");
});

const resize = () => refit((cols, rows) => Call.ByName(WINDOWS_SERVICE + ".Resize", id, cols, rows));
watchSize(host, resize);

await Call.ByName(WINDOWS_SERVICE + ".Start", id, term.cols, term.rows);
term.focus();
