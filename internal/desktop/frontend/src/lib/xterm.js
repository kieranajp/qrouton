import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import "@xterm/xterm/css/xterm.css";

// getPropertyValue does not resolve a var() chain, so read the ramp itself.
const shade = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();

// The agent reads a lone LF as submit, but an LF inside a bracketed paste as a
// literal newline.
const SHIFT_ENTER = "\x1b[200~\n\x1b[201~";

const encoder = new TextEncoder();

/** Base64 encodes text, because a raw PTY frame is not valid JSON. */
export const encode = (text) => btoa(String.fromCharCode(...encoder.encode(text)));

/** decode turns one base64 PTY frame back into the bytes xterm expects. */
export function decode(encoded) {
  const raw = atob(encoded);
  const buffer = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) buffer[i] = raw.charCodeAt(i);
  return buffer;
}

/**
 * mount attaches a terminal to host and wires its input to write. It returns
 * the terminal and a refit callback the caller reports to its own Go service:
 * the two window kinds name different services for the same three methods.
 *
 * @param {HTMLElement} host
 * @param {{write: (text: string) => void, background?: string}} options
 */
export function mount(host, { write, background = "--ctp-base" }) {
  const term = new Terminal({
    fontFamily: 'Menlo, ui-monospace, "JetBrains Mono Variable", monospace',
    fontSize: 13,
    allowProposedApi: true,
    macOptionIsMeta: true,
    theme: {
      background: shade(background),
      foreground: shade("--ctp-text"),
      cursor: shade("--ctp-rosewater"),
    },
  });
  const fit = new FitAddon();
  term.loadAddon(fit);
  term.open(host);
  try {
    term.loadAddon(new WebglAddon());
  } catch (e) {
    console.warn("webgl", e);
  }
  fit.fit();

  term.attachCustomKeyEventHandler((event) => {
    if (event.type !== "keydown") return true;
    if (event.key === "Enter" && event.shiftKey) {
      // Returning false stops xterm's keydown handling but not the browser's
      // own keypress, which would append a CR and submit.
      event.preventDefault();
      event.stopPropagation();
      write(SHIFT_ENTER);
      return false;
    }
    return true;
  });
  term.onData((data) => write(data));

  const refit = (report) => {
    fit.fit();
    report(term.cols, term.rows);
  };
  return { term, fit, refit };
}

/** watchSize refits the terminal whenever its host or the window changes size. */
export function watchSize(host, resize) {
  new ResizeObserver(resize).observe(host);
  window.addEventListener("resize", resize);
}
