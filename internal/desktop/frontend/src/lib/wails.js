// Served by the workbench rather than bundled: an npm copy drifting from the Go
// module fails silently in a webview with no console.
import { Browser, Clipboard } from "/wails/runtime.js";

export { Call, Events } from "/wails/runtime.js";

export const openURL = (url) => Browser.OpenURL(url).catch(() => {});
export const copyText = (text) => Clipboard.SetText(text);
// A clipboard the webview refuses to read is an empty one, not a failed paste.
export const clipboardText = () => Clipboard.Text().catch(() => "");
