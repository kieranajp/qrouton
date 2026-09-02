import assert from "node:assert/strict";
import test from "node:test";
import {
  DEFAULT_STICKER_LABELS,
  STICKERS,
  sticker,
  stickerControlLabel,
  stickerFeedback,
  stickerLabel,
  stickerText,
  stickerTitle,
} from "./stickers.js";

test("none and unknown IDs are treated as no sticker", () => {
  for (const id of ["", undefined, "future-shape"]) {
    assert.equal(sticker(id), null);
    assert.equal(stickerText(id), "No sticker");
    assert.equal(stickerFeedback(id), "No sticker");
    assert.equal(stickerTitle(id), "Set sticker");
    assert.equal(
      stickerControlLabel("Checkout", id),
      "Set sticker for Checkout; current sticker: No sticker",
    );
  }
});

test("the catalogue fixes all four semantic presentations", () => {
  assert.deepEqual(
    Object.values(STICKERS).map(({ id, colour, shape, css }) => [id, colour, shape, css]),
    [
      ["star", "Blue", "star", "var(--sticker-blue)"],
      ["bookmark", "Green", "bookmark", "var(--sticker-green)"],
      ["question", "Orange", "question mark", "var(--sticker-orange)"],
      ["exclamation", "Red", "exclamation mark", "var(--sticker-red)"],
    ],
  );
  assert.equal(stickerText("star"), "Blue star — Important");
  assert.equal(stickerText("bookmark"), "Green bookmark — Read later");
  assert.equal(stickerText("question"), "Orange question mark — Needs follow-up");
  assert.equal(stickerText("exclamation"), "Red exclamation mark — Has bugs");
});

test("configured labels replace defaults member by member", () => {
  const labels = { star: "Urgent", question: "Ask Kieran" };
  assert.equal(stickerLabel("star", labels), "Urgent");
  assert.equal(stickerLabel("bookmark", labels), DEFAULT_STICKER_LABELS.bookmark);
  assert.equal(stickerFeedback("question", labels), "Ask Kieran");
  assert.equal(stickerTitle("question", labels), "Ask Kieran");
  assert.equal(
    stickerControlLabel("Checkout", "exclamation", { exclamation: "Broken" }),
    "Change sticker for Checkout; current sticker: Red exclamation mark — Broken",
  );
});
