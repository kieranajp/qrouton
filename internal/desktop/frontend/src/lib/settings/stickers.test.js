import assert from "node:assert/strict";
import test from "node:test";
import { addSticker, duplicates, firstUnusedPair, removeSticker, toInput, updateSticker } from "./stickers.js";

const seed = [
  { id: "star", colour: "blue", symbol: "star", label: "Important" },
  { id: "bookmark", colour: "green", symbol: "bookmark", label: "Read later" },
];

test("a new sticker takes the first unused pair, trying every colour of a symbol first", () => {
  assert.deepEqual(firstUnusedPair([]), { colour: "blue", symbol: "star" });
  assert.deepEqual(firstUnusedPair(seed), { colour: "sapphire", symbol: "star" });
  const added = addSticker(seed);
  assert.equal(added.length, 3);
  assert.deepEqual(added[2], { id: "", colour: "sapphire", symbol: "star", label: "" });
  assert.equal(seed.length, 2);
});

test("remove and update leave the other rows alone", () => {
  assert.deepEqual(removeSticker(seed, 0), [seed[1]]);
  const updated = updateSticker(seed, 1, { colour: "pink", symbol: "heart" });
  assert.deepEqual(updated[1], { id: "bookmark", colour: "pink", symbol: "heart", label: "Read later" });
  assert.equal(updated[0], seed[0]);
});

test("only a repeat of an earlier pair counts as a duplicate", () => {
  const list = [...seed, { id: "", colour: "blue", symbol: "star", label: "Again" }, { id: "", colour: "blue", symbol: "star", label: "" }];
  assert.deepEqual([...duplicates(list)], [2, 3]);
  assert.deepEqual([...duplicates(seed)], []);
});

test("the Save payload carries only the four sticker fields", () => {
  assert.deepEqual(toInput([/** @type {any} */ ({ ...seed[0], extra: true })]), [seed[0]]);
});
