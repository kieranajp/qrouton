import { Call } from "./wails.js";

const SESSIONS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Sessions";

export const show = (slug) => Call.ByName(SESSIONS_SERVICE + ".Show", slug);
export const reveal = (slug) => Call.ByName(SESSIONS_SERVICE + ".Reveal", slug);
export const uncommitted = (slug) => Call.ByName(SESSIONS_SERVICE + ".Uncommitted", slug);
export const cleanup = (slug) => Call.ByName(SESSIONS_SERVICE + ".Cleanup", slug);
