import assert from "node:assert/strict";
import test from "node:test";
import { clock, relative, stamp } from "./relative.js";

// Half an hour off UTC, so a formatter reading UTC hours or minutes cannot pass here.
process.env.TZ = "Asia/Kolkata";

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
    assert.equal(relative(/** @type {string} */ (/** @type {unknown} */ (undefined)), style, NOW), "");
  }
});

// A clock behind the one that wrote the timestamp must not read as the future.
test("a time still ahead of now reads as just now", () => {
  assert.equal(relative(ago(-5 * MINUTE), "compact", NOW), "just now");
  assert.equal(relative(ago(-5 * MINUTE), "prose", NOW), "just now");
});

test("an ago age says just now, then counts minutes up to weeks and months", () => {
  assert.equal(relative(ago(30000), "ago", NOW), "just now");
  assert.equal(relative(ago(4 * MINUTE), "ago", NOW), "4m ago");
  assert.equal(relative(ago(7 * DAY), "ago", NOW), "1w ago");
  assert.equal(relative(ago(61 * DAY), "ago", NOW), "2mo ago");
  assert.equal(relative("", "ago", NOW), "");
});

// A twelve-hour or UTC clock would show a time the user's wall clock never read.
test("a clock time is the local hour and minute, padded, on a 24-hour face", () => {
  assert.equal(clock(Date.UTC(2026, 7, 12, 3, 11, 59)), "08:41");
  assert.equal(clock(Date.UTC(2026, 7, 12, 17, 35)), "23:05");
});

// A clock time for never would read as "05:53", the year-one instant in this zone.
test("a clock time for never says nothing", () => {
  assert.equal(clock("0001-01-01T00:00:00Z"), "");
  assert.equal(clock(""), "");
  assert.equal(stamp("0001-01-01T00:00:00Z"), "");
});

// A cache kept overnight would otherwise say "As of 17:30" the next morning, as if it were today's.
test("a stamp is the clock time today and carries a short date on any other local day", () => {
  const noon = Date.UTC(2026, 10, 20, 6, 30);
  assert.equal(stamp(Date.UTC(2026, 10, 20, 3, 11), noon), "08:41");
  assert.equal(stamp(Date.UTC(2026, 10, 18, 12, 0), noon), "18 Nov 17:30");
  assert.equal(stamp(Date.UTC(2026, 10, 20, 19, 0), noon), "21 Nov 00:30");
});

// Comparing UTC days would date a time that is already today on the user's wall clock.
test("a stamp compares local days, not UTC ones", () => {
  assert.equal(stamp(Date.UTC(2026, 10, 19, 19, 0), Date.UTC(2026, 10, 20, 6, 30)), "00:30");
});
