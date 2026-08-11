// Served by the workbench rather than bundled: an npm copy drifting from the Go
// module fails silently in a webview with no console.
import { Browser } from "/wails/runtime.js";

export { Call, Events } from "/wails/runtime.js";

export const openURL = (url) => Browser.OpenURL(url).catch(() => {});
