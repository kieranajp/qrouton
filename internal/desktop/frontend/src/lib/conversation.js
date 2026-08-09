import { Call, Events } from "./wails.js";
import { decode, encode, mount, watchSize } from "./xterm.js";

const TERM_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Term";

/** attach runs the conversation's PTY in host and keeps it sized to it. */
export async function attach(host) {
  const write = (text) => Call.ByName(TERM_SERVICE + ".Write", encode(text));
  const { term, refit } = mount(host, { write, background: "--ctp-crust" });

  term.onBinary((data) => Call.ByName(TERM_SERVICE + ".Write", btoa(data)));

  // Emit(name, one) delivers the value itself; indexing it would take the first
  // character of the base64 string and fail silently in here.
  Events.On("pty:data", (event) => term.write(decode(event.data)));

  // A clean exit takes the window with it, so this is only read after a failure.
  Events.On("pty:exit", (event) => {
    term.write("\r\n\x1b[2m[session ended — status " + event.data + "]\x1b[0m\r\n");
  });

  watchSize(host, () => refit((cols, rows) => Call.ByName(TERM_SERVICE + ".Resize", cols, rows)));
  await Call.ByName(TERM_SERVICE + ".Start", term.cols, term.rows);
  term.focus();
}
