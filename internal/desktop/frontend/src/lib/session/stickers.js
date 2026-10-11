/** @typedef {{name: string, fill?: string, stroke?: string, width?: number}} StickerSymbol */

/** @type {Readonly<Record<string, StickerSymbol>>} */
export const STICKER_SYMBOLS = Object.freeze({
  star: Object.freeze({ name: "star", fill: "m8 1.5 1.9 3.85 4.25.62-3.08 3 0.73 4.23L8 11.2l-3.8 2 .73-4.23-3.08-3 4.25-.62L8 1.5Z" }),
  bookmark: Object.freeze({ name: "bookmark", fill: "M4 2.25h8v11.5L8 11.2l-4 2.55V2.25Z" }),
  question: Object.freeze({
    name: "question mark",
    fill: "M7.15 12.6a1.1 1.1 0 1 0 2.2 0a1.1 1.1 0 1 0-2.2 0Z",
    stroke: "M5.25 5.35A2.8 2.8 0 0 1 8.1 2.7c1.7 0 2.9 1.05 2.9 2.55 0 1.22-.68 1.87-1.64 2.48-.84.53-1.11.9-1.11 1.72",
    width: 2,
  }),
  exclamation: Object.freeze({
    name: "exclamation mark",
    fill: "M6.9 12.6a1.1 1.1 0 1 0 2.2 0a1.1 1.1 0 1 0-2.2 0Z",
    stroke: "M8 2.5v7",
    width: 2.2,
  }),
  dot: Object.freeze({ name: "dot", fill: "M3 8a5 5 0 1 0 10 0a5 5 0 1 0-10 0Z" }),
  square: Object.freeze({ name: "square", fill: "M3.25 3.25h9.5v9.5h-9.5Z" }),
  diamond: Object.freeze({ name: "diamond", fill: "M8 1.75 14.25 8 8 14.25 1.75 8Z" }),
  triangle: Object.freeze({ name: "triangle", fill: "M8 2.25 14.5 13.5h-13Z" }),
  heart: Object.freeze({ name: "heart", fill: "M8 13.75 2.7 8.6A3.15 3.15 0 0 1 8 4.15a3.15 3.15 0 0 1 5.3 4.45Z" }),
  bolt: Object.freeze({ name: "bolt", fill: "M9.75 1.25 3.5 9h4.1l-1.35 5.75L12.5 7H8.4Z" }),
  flag: Object.freeze({ name: "flag", fill: "M3.25 1.75h1.6v12.5h-1.6ZM4.85 2.5h7.9l-2.1 3.1 2.1 3.1h-7.9Z" }),
  check: Object.freeze({ name: "tick", stroke: "M3 8.5l3 3 7-7", width: 2.2 }),
  cross: Object.freeze({ name: "cross", stroke: "M4 4l8 8M12 4l-8 8", width: 2.2 }),
  pause: Object.freeze({ name: "pause", fill: "M4 3h3v10H4ZM9 3h3v10H9Z" }),
});

/** @typedef {{id: string, colour: string, symbol: string, label: string}} Sticker */

/** An empty id, or one the list no longer holds, is no sticker.
 * @param {string | undefined} id
 * @param {Sticker[]} [stickers]
 * @returns {Sticker | null} */
export const sticker = (id, stickers = []) => (id ? (stickers ?? []).find((s) => s.id === id) ?? null : null);

const CSS = Object.freeze({
  blue: "var(--sticker-blue)",
  sapphire: "var(--sticker-sapphire)",
  teal: "var(--sticker-teal)",
  green: "var(--sticker-green)",
  yellow: "var(--sticker-yellow)",
  orange: "var(--sticker-orange)",
  red: "var(--sticker-red)",
  maroon: "var(--sticker-maroon)",
  pink: "var(--sticker-pink)",
  mauve: "var(--sticker-mauve)",
  lavender: "var(--sticker-lavender)",
  grey: "var(--sticker-grey)",
});

export const STICKER_COLOURS = Object.freeze(Object.keys(CSS));

/** @param {string} colour */
export const stickerCss = (colour) => CSS[colour] ?? "currentColor";

/** @param {{colour: string, symbol: string}} item */
export const stickerName = (item) =>
  `${item.colour.charAt(0).toUpperCase()}${item.colour.slice(1)} ${STICKER_SYMBOLS[item.symbol]?.name ?? item.symbol}`;

export const stickerText = (id, stickers = []) => {
  const item = sticker(id, stickers);
  return item ? `${stickerName(item)} — ${item.label}` : "No sticker";
};

export const stickerControlLabel = (name, id, stickers = []) =>
  `${sticker(id, stickers) ? "Change" : "Set"} sticker for ${name}; current sticker: ${stickerText(id, stickers)}`;

export const stickerTitle = (id, stickers = []) => sticker(id, stickers)?.label ?? "Set sticker";

export const stickerFeedback = (id, stickers = []) => sticker(id, stickers)?.label ?? "No sticker";
