import assert from "node:assert/strict";
import test from "node:test";
import { relative } from "./relative.js";

const NOW = Date.parse("2026-08-12T12:00:00Z");
const ago = (ms) => new Date(NOW - ms).toISOString();

const MINUTE = 60000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

test("a compact age counts the minutes a prose age will not", () => {
  assert.equal(relative(ago(4 * MINUTE), "compact", NOW), "4m ago");
  assert.equal(relative(ago(4 * MINUTE), "prose", NOW), "just now");
  assert.equal(relative(ago(59 * MINUTE), "compact", NOW), "59m ago");
});

test("under a minute is not dressed up as a duration", () => {
  assert.equal(relative(ago(30000), "compact", NOW), "just now");
  assert.equal(relative(ago(30000), "prose", NOW), "just now");
});

test("both styles say hours the same way", () => {
  assert.equal(relative(ago(HOUR), "compact", NOW), "1h ago");
  assert.equal(relative(ago(HOUR), "prose", NOW), "1h ago");
  assert.equal(relative(ago(23 * HOUR), "compact", NOW), "23h ago");
  assert.equal(relative(ago(23 * HOUR), "prose", NOW), "23h ago");
});

test("days are a letter in compact and a word in prose, singular where they are", () => {
  assert.equal(relative(ago(DAY), "compact", NOW), "1d ago");
  assert.equal(relative(ago(DAY), "prose", NOW), "1 day ago");
  assert.equal(relative(ago(30 * DAY), "compact", NOW), "30d ago");
  assert.equal(relative(ago(30 * DAY), "prose", NOW), "30 days ago");
});

// A document written in a long-running session stays a count; a repository row
// has a date to fall back on once the count stops meaning anything.
test("only prose gives up counting days and carries a date instead", () => {
  assert.equal(relative("2026-03-14T09:00:00Z", "prose", NOW), "2026-03-14");
  assert.equal(relative("2026-03-14T09:00:00Z", "compact", NOW), "151d ago");
});

test("nothing behind it says nothing, in either style", () => {
  for (const style of /** @type {const} */ (["compact", "prose"])) {
    assert.equal(relative("", style, NOW), "");
    assert.equal(relative("0001-01-01T00:00:00Z", style, NOW), "");
    assert.equal(relative(undefined, style, NOW), "");
  }
});

// A clock behind the one that wrote the timestamp must not read as the future.
test("a time still ahead of now reads as just now", () => {
  assert.equal(relative(ago(-5 * MINUTE), "compact", NOW), "just now");
  assert.equal(relative(ago(-5 * MINUTE), "prose", NOW), "just now");
});
