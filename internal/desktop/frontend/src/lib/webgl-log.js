import { INSPECT_QUERY } from "./bridge/generated.js";

const noWatch = () => () => {};

/** webglWatcher records nothing unless the page was opened for inspection.
 * @param {string} search @param {(event: string, terminal: string, pane: string, live: number) => void} send */
export function webglWatcher(search, send) {
  if (!new URLSearchParams(search).has(INSPECT_QUERY)) return noWatch;
  let live = 0;
  /** @param {{host: EventTarget, addon: {onContextLoss: (listener: () => void) => {dispose: () => void}}, terminal: string, pane: string}} renderer */
  return ({ host, addon, terminal, pane }) => {
    const record = (event) => send(event, terminal, pane, live);
    // Context events fire on the canvas without bubbling, so the host hears them only while capturing.
    const listening = new AbortController();
    const options = { capture: true, signal: listening.signal };
    host.addEventListener("webglcontextlost", () => record("lost"), options);
    host.addEventListener("webglcontextrestored", () => record("restored"), options);
    const unrestored = addon.onContextLoss(() => record("unrestored"));
    live++;
    record("created");
    return () => {
      listening.abort();
      unrestored.dispose();
      live--;
      record("disposed");
    };
  };
}
