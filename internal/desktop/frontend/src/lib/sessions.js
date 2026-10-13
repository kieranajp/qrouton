import {
  SESSIONS_CLEANUP,
  SESSIONS_COPY_ASSET,
  SESSIONS_CYCLE_STICKER,
  SESSIONS_RELOAD,
  SESSIONS_REORDER,
  SESSIONS_REVEAL,
  SESSIONS_REVEAL_PATH,
  SESSIONS_SHOW,
  SESSIONS_UNCOMMITTED,
  SESSIONS_UNLOAD,
} from "./bridge/generated.js";
import { Call } from "./wails.js";

export const show = (slug) => Call.ByName(SESSIONS_SHOW, slug);
export const cycleSticker = (slug) => Call.ByName(SESSIONS_CYCLE_STICKER, slug);
export const unload = (slug) => Call.ByName(SESSIONS_UNLOAD, slug);
export const reorder = (slug, targetSlug) => Call.ByName(SESSIONS_REORDER, slug, targetSlug);
export const reload = (slug) => Call.ByName(SESSIONS_RELOAD, slug);
export const reveal = (slug) => Call.ByName(SESSIONS_REVEAL, slug);
export const revealPath = (slug, path) => Call.ByName(SESSIONS_REVEAL_PATH, slug, path);
export const uncommitted = (slug) => Call.ByName(SESSIONS_UNCOMMITTED, slug);
export const cleanup = (slug) => Call.ByName(SESSIONS_CLEANUP, slug);
export const copyAsset = (slug, path) => Call.ByName(SESSIONS_COPY_ASSET, slug, path);
