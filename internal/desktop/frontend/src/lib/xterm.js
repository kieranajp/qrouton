import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import "@xterm/xterm/css/xterm.css";
import { latestPerFrame } from "./frame.js";
import { opensSettings, position } from "./shortcuts.js";

export { Terminal };

// getPropertyValue does not resolve a var() chain, so read the ramp itself.
const shade = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();

// The agent reads a lone LF as submit, but an LF inside a bracketed paste as a
// literal newline.
const SHIFT_ENTER = "\x1b[200~\n\x1b[201~";

const encoder = new TextEncoder();
const CHUNK = 0x8000;

// String.fromCharCode(...bytes) spreads the array as call arguments, which
// throws past ~128KB; chunking keeps every paste size working.
export const encode = (text) => {
  const bytes = encoder.encode(text);
  let binary = "";
  for (let i = 0; i < bytes.length; i += CHUNK) {
    binary += String.fromCharCode(...bytes.subarray(i, i + CHUNK));
  }
  return btoa(binary);
};

export function decode(encoded) {
  const raw = atob(encoded);
  const buffer = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) buffer[i] = raw.charCodeAt(i);
  return buffer;
}

/** A retained replay replaces remounted terminal contents; ordinary chunks append.
 * @param {Terminal} term
 * @param {string | {encoded: string, replay?: boolean}} payload */
export function paint(term, payload) {
  const chunk = typeof payload === "string" ? { encoded: payload } : payload;
  // Keep the reset in xterm's write queue. Calling reset() synchronously could
  // overtake an ordinary chunk which the parser has accepted but not painted.
  if (chunk.replay) term.write("\x1bc");
  term.write(decode(chunk.encoded));
}

const mounted = new WeakMap();

/** terminalAt is the terminal a node sits inside, and undefined outside one. */
export function terminalAt(node) {
  for (let element = node; element; element = element.parentElement) {
    const term = mounted.get(element);
    if (term) return term;
  }
  return undefined;
}

/**
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
  mounted.set(host, term);
  try {
    term.loadAddon(new WebglAddon());
  } catch (e) {
    console.warn("webgl", e);
  }
  fit.fit();

  term.attachCustomKeyEventHandler((event) => {
    if (event.type !== "keydown") return true;
    // Returning false without preventing the default leaves the keystroke to
    // bubble, which is how the session shortcut reaches the page from in here.
    if (position(event) || opensSettings(event)) return false;
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

  let { cols, rows } = term;
  // Reporting is a SIGWINCH to the child, and a drag fires far more often than
  // it crosses a cell.
  const refit = (report) => {
    fit.fit();
    if (term.cols === cols && term.rows === rows) return;
    ({ cols, rows } = term);
    report(cols, rows);
  };
  const dispose = () => {
    mounted.delete(host);
    term.dispose();
  };
  return { term, fit, refit, dispose };
}

export function watchSize(host, resize) {
  let live = true;
  const scheduled = latestPerFrame(() => resize());
  const notify = () => {
    if (live) scheduled.schedule();
  };
  const observer = new ResizeObserver(notify);
  observer.observe(host);
  window.addEventListener("resize", notify);
  return () => {
    live = false;
    scheduled.cancel();
    observer.disconnect();
    window.removeEventListener("resize", notify);
  };
}
