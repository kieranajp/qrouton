export const DEFAULT_STICKER_LABELS = Object.freeze({
  star: "Important",
  bookmark: "Read later",
  question: "Needs follow-up",
  exclamation: "Has bugs",
});

export const STICKERS = Object.freeze({
  star: Object.freeze({ id: "star", colour: "Blue", shape: "star", css: "var(--sticker-blue)" }),
  bookmark: Object.freeze({
    id: "bookmark",
    colour: "Green",
    shape: "bookmark",
    css: "var(--sticker-green)",
  }),
  question: Object.freeze({
    id: "question",
    colour: "Orange",
    shape: "question mark",
    css: "var(--sticker-orange)",
  }),
  exclamation: Object.freeze({
    id: "exclamation",
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
