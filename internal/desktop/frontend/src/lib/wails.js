// Served by the workbench rather than bundled: an npm copy drifting from the Go
// module fails silently in a webview with no console.
import { Browser, Clipboard, Events } from "/wails/runtime.js";
import { isAllowedURL } from "./urlpolicy.js";

export { Call, Events } from "/wails/runtime.js";

export const openURL = (url) =>
  (isAllowedURL(url) ? Browser.OpenURL(url) : Promise.resolve()).catch(() => {});
export const copyText = (text) => Clipboard.SetText(text);
// A clipboard the webview refuses to read is an empty one, not a failed paste.
export const clipboardText = () => Clipboard.Text().catch(() => "");

/** Both union arms expose both keys for callers without strict null checks.
 * @template T
 * @typedef {{ok: true, value: T, error?: undefined} | {ok: false, value?: undefined, error: any}} Answer */

/** Bridge failures resolve as values so the UI cannot drop unhandled rejections.
 * @template T
 * @param {Promise<T>} answering
 * @returns {Promise<Answer<T>>} */
export const call = (answering) =>
  answering.then(
    (value) => ({ ok: true, value }),
    (error) => ({ ok: false, error }),
  );

/** Subscribes, then snapshots. A snapshot that answers after an event is older than it, so it is dropped.
 * @template T @param {string} event @param {() => Promise<Answer<T>>} snapshot @param {(value: T | undefined) => void} apply
 * @param {{accepts?: (data: any) => boolean, settle?: (answer: Answer<T>, live: boolean) => void}} [options]
 * @returns {{stop: () => void, refresh: () => void}} */
export function follow(event, snapshot, apply, { accepts = () => true, settle } = {}) {
  let events = 0;
  let stopped = false;
  const off = Events.On(event, (/** @type {{data: any}} */ incoming) => {
    if (stopped || !accepts(incoming.data)) return;
    events++;
    apply(incoming.data);
  });
  const refresh = () => {
    const before = events;
    snapshot().then((answer) => {
      if (stopped) return;
      const live = events !== before;
      if (settle) settle(answer, live);
      else if (!live) apply(answer.ok ? answer.value : undefined);
    });
  };
  refresh();
  return {
    stop: () => {
      stopped = true;
      off();
    },
    refresh,
  };
}
