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

  let { cols, rows } = term;
  // Reporting is a SIGWINCH to the child, and a drag fires far more often than
  // it crosses a cell.
  const refit = (report) => {
    fit.fit();
    if (term.cols === cols && term.rows === rows) return;
    ({ cols, rows } = term);
    report(cols, rows);
  };
  return { term, fit, refit, dispose: () => term.dispose() };
}

export function watchSize(host, resize) {
  const observer = new ResizeObserver(resize);
  observer.observe(host);
  window.addEventListener("resize", resize);
  return () => {
    observer.disconnect();
    window.removeEventListener("resize", resize);
  };
}
