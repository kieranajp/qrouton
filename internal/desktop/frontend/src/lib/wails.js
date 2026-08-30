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

/**
 * Answer is one bridge call's outcome. Both halves carry both keys so the union
 * reads the same with or without strict null checks.
 * @template T
 * @typedef {{ok: true, value: T, error?: undefined} | {ok: false, value?: undefined, error: any}} Answer
 */

/**
 * call is how a view-model asks the workbench something. The answer is a value
 * either way: a rejection nobody reads is silently nothing in a webview with no
 * console.
 * @template T
 * @param {Promise<T>} answering
 * @returns {Promise<Answer<T>>}
 */
export const call = (answering) =>
  answering.then(
    (value) => ({ ok: true, value }),
    (error) => ({ ok: false, error }),
  );
