import {
  SESSIONS_CLEANUP,
  SESSIONS_REVEAL,
  SESSIONS_SHOW,
  SESSIONS_UNCOMMITTED,
} from "./bridge/generated.js";
import { Call } from "./wails.js";

export const show = (slug) => Call.ByName(SESSIONS_SHOW, slug);
export const reveal = (slug) => Call.ByName(SESSIONS_REVEAL, slug);
export const uncommitted = (slug) => Call.ByName(SESSIONS_UNCOMMITTED, slug);
export const cleanup = (slug) => Call.ByName(SESSIONS_CLEANUP, slug);
