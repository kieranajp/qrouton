import { STICKER_COLOURS, STICKER_SYMBOLS } from "../bridge/generated.js";

/** @typedef {import("../session/stickers.js").Sticker} Sticker */

/** Symbols outer and colours inner, so new stickers try every colour of a star first.
 * @param {Sticker[]} list */
export function firstUnusedPair(list) {
  for (const symbol of STICKER_SYMBOLS) {
    for (const colour of STICKER_COLOURS) {
      if (!list.some((item) => item.colour === colour && item.symbol === symbol)) return { colour, symbol };
    }
  }
  return { colour: "mauve", symbol: "dot" };
}

/** A new sticker has no id; Save gives it one.
 * @param {Sticker[]} list */
export const addSticker = (list) => [...list, { id: "", ...firstUnusedPair(list), label: "" }];

/** @param {Sticker[]} list @param {number} index */
export const removeSticker = (list, index) => list.filter((_, i) => i !== index);

/** @param {Sticker[]} list @param {number} index @param {Partial<Sticker>} patch */
export const updateSticker = (list, index, patch) => list.map((item, i) => (i === index ? { ...item, ...patch } : item));

/** The rows Save would refuse: each repeat of a pair an earlier row already has.
 * @param {Sticker[]} list
 * @returns {Set<number>} */
export function duplicates(list) {
  const seen = new Set();
  const out = new Set();
  list.forEach((item, index) => {
    const pair = `${item.colour}/${item.symbol}`;
    if (seen.has(pair)) out.add(index);
    seen.add(pair);
  });
  return out;
}

/** @param {Sticker[]} list */
export const toInput = (list) => list.map(({ id, colour, symbol, label }) => ({ id, colour, symbol, label }));
