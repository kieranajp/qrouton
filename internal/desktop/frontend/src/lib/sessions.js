import {
  SESSIONS_CLEANUP,
  SESSIONS_CYCLE_STICKER,
  SESSIONS_RELOAD,
  SESSIONS_REVEAL,
  SESSIONS_REVEAL_PATH,
  SESSIONS_SHOW,
  SESSIONS_UNCOMMITTED,
} from "./bridge/generated.js";
import { Call } from "./wails.js";

export const show = (slug) => Call.ByName(SESSIONS_SHOW, slug);
export const cycleSticker = (slug) => Call.ByName(SESSIONS_CYCLE_STICKER, slug);
export const reload = (slug) => Call.ByName(SESSIONS_RELOAD, slug);
export const reveal = (slug) => Call.ByName(SESSIONS_REVEAL, slug);
export const revealPath = (slug, path) => Call.ByName(SESSIONS_REVEAL_PATH, slug, path);
export const uncommitted = (slug) => Call.ByName(SESSIONS_UNCOMMITTED, slug);
export const cleanup = (slug) => Call.ByName(SESSIONS_CLEANUP, slug);
