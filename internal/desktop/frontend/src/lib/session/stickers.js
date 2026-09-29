import { STICKER_IDS } from "../bridge/generated.js";

export const DEFAULT_STICKER_LABELS = Object.freeze({
  [STICKER_IDS.STAR]: "Important",
  [STICKER_IDS.BOOKMARK]: "Read later",
  [STICKER_IDS.QUESTION]: "Needs follow-up",
  [STICKER_IDS.EXCLAMATION]: "Has bugs",
});

export const STICKERS = Object.freeze({
  [STICKER_IDS.STAR]: Object.freeze({ id: STICKER_IDS.STAR, colour: "Blue", shape: "star", css: "var(--sticker-blue)" }),
  [STICKER_IDS.BOOKMARK]: Object.freeze({
    id: STICKER_IDS.BOOKMARK,
    colour: "Green",
    shape: "bookmark",
    css: "var(--sticker-green)",
  }),
  [STICKER_IDS.QUESTION]: Object.freeze({
    id: STICKER_IDS.QUESTION,
    colour: "Orange",
    shape: "question mark",
    css: "var(--sticker-orange)",
  }),
  [STICKER_IDS.EXCLAMATION]: Object.freeze({
    id: STICKER_IDS.EXCLAMATION,
    colour: "Red",
    shape: "exclamation mark",
    css: "var(--sticker-red)",
  }),
});

export const sticker = (id) => STICKERS[id] ?? null;

export const stickerLabel = (id, labels = {}) =>
  sticker(id) ? labels?.[id] || DEFAULT_STICKER_LABELS[id] : "";

export const stickerText = (id, labels = {}) => {
  const item = sticker(id);
  return item ? `${item.colour} ${item.shape} — ${stickerLabel(id, labels)}` : "No sticker";
};

export const stickerControlLabel = (name, id, labels = {}) =>
  `${sticker(id) ? "Change" : "Set"} sticker for ${name}; current sticker: ${stickerText(id, labels)}`;

export const stickerTitle = (id, labels = {}) =>
  sticker(id) ? stickerLabel(id, labels) : "Set sticker";

export const stickerFeedback = (id, labels = {}) =>
  sticker(id) ? stickerLabel(id, labels) : "No sticker";
