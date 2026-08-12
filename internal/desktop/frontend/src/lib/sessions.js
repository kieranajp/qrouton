import { Call } from "./wails.js";

const SESSIONS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Sessions";

export const show = (slug) => Call.ByName(SESSIONS_SERVICE + ".Show", slug);
