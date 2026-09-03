// Served by the workbench rather than bundled: an npm copy drifting from the Go
// module fails silently in a webview with no console.
import { Browser, Clipboard } from "/wails/runtime.js";
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
