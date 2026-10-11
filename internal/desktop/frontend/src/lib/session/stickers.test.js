import assert from "node:assert/strict";
import test from "node:test";
import {
  STICKER_COLOURS,
  STICKER_SYMBOLS,
  sticker,
  stickerControlLabel,
  stickerCss,
  stickerFeedback,
  stickerName,
  stickerText,
  stickerTitle,
} from "./stickers.js";

const stickers = [
  { id: "star", colour: "blue", symbol: "star", label: "Important" },
  { id: "question", colour: "orange", symbol: "question", label: "Needs follow-up" },
  { id: "a1b2c3d4", colour: "pink", symbol: "check", label: "Done" },
];

test("none, unknown and removed ids are treated as no sticker", () => {
  for (const id of ["", undefined, "future-shape", "bookmark"]) {
    assert.equal(sticker(id, stickers), null);
    assert.equal(stickerText(id, stickers), "No sticker");
    assert.equal(stickerFeedback(id, stickers), "No sticker");
    assert.equal(stickerTitle(id, stickers), "Set sticker");
    assert.equal(stickerControlLabel("Checkout", id, stickers), "Set sticker for Checkout; current sticker: No sticker");
  }
});

test("a listed id reads its colour, symbol and label from the list", () => {
  assert.equal(stickerText("star", stickers), "Blue star — Important");
  assert.equal(stickerText("question", stickers), "Orange question mark — Needs follow-up");
  assert.equal(stickerText("a1b2c3d4", stickers), "Pink tick — Done");
  assert.equal(stickerTitle("question", stickers), "Needs follow-up");
  assert.equal(
    stickerControlLabel("Checkout", "star", stickers),
    "Change sticker for Checkout; current sticker: Blue star — Important",
  );
  assert.equal(stickerName({ colour: "grey", symbol: "exclamation" }), "Grey exclamation mark");
  assert.equal(stickerCss("lavender"), "var(--sticker-lavender)");
});

test("the palette holds twelve colours and fourteen drawable symbols", () => {
  assert.equal(STICKER_COLOURS.length, 12);
  assert.equal(Object.keys(STICKER_SYMBOLS).length, 14);
  for (const symbol of Object.values(STICKER_SYMBOLS)) assert.ok(symbol.fill || symbol.stroke);
});
