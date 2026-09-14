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

const WINDOW_REPORTS = new Set([11, 13, 14, 15, 16, 18, 19, 20, 21]);

const first = (params) => params[0] ?? 0;
const reportsWindowState = (params) =>
  WINDOW_REPORTS.has(first(params)) && !(first(params) === 14 && params[1] === 2);

export function createTerminalPainter(term) {
  let replaying = false;
  let writing = false;
  let live = true;
  const queue = [];
  const guards = [
    term.parser.registerCsiHandler({ final: "c" }, (params) => replaying && first(params) === 0),
    term.parser.registerCsiHandler({ prefix: ">", final: "c" }, (params) => replaying && first(params) === 0),
    term.parser.registerCsiHandler({ intermediates: "$", final: "p" }, () => replaying),
    term.parser.registerCsiHandler({ prefix: "?", intermediates: "$", final: "p" }, () => replaying),
    term.parser.registerCsiHandler({ final: "n" }, (params) => replaying && [5, 6].includes(first(params))),
    term.parser.registerCsiHandler({ prefix: "?", final: "n" }, (params) => replaying && first(params) === 6),
    term.parser.registerCsiHandler({ final: "t" }, (params) => replaying && reportsWindowState(params)),
    term.parser.registerDcsHandler({ intermediates: "$", final: "q" }, () => replaying),
  ];

  const next = () => {
    if (!live || writing || queue.length === 0) return;
    writing = true;
    const chunk = queue.shift();
    if (!chunk.replay) {
      term.write(decode(chunk.encoded), () => {
        if (!live) return;
        writing = false;
        next();
      });
      return;
    }

    replaying = true;
    term.write("\x1bc", () => {
      if (!live) return;
      term.write(decode(chunk.encoded), () => {
        if (!live) return;
        replaying = false;
        writing = false;
        next();
      });
    });
  };

  return {
    paint(payload) {
      if (!live) return;
      queue.push(typeof payload === "string" ? { encoded: payload } : payload);
      next();
    },
    dispose() {
      if (!live) return;
      live = false;
      replaying = false;
      queue.length = 0;
      for (const guard of guards) guard.dispose();
    },
  };
}
