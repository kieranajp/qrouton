import assert from "node:assert/strict";
import test from "node:test";
import { pushed } from "./pushed.js";

const NOW = Date.parse("2026-08-12T12:00:00Z");
const ago = (ms) => new Date(NOW - ms).toISOString();

const HOUR = 3600000;
const DAY = 24 * HOUR;

test("a push within the hour is not dressed up as a duration", () => {
  assert.equal(pushed(ago(4 * 60000), NOW), "pushed just now");
});

test("hours read as hours and days as days, singular where they are", () => {
  assert.equal(pushed(ago(2 * HOUR), NOW), "pushed 2h ago");
  assert.equal(pushed(ago(23 * HOUR), NOW), "pushed 23h ago");
  assert.equal(pushed(ago(DAY), NOW), "pushed 1 day ago");
  assert.equal(pushed(ago(30 * DAY), NOW), "pushed 30 days ago");
});

test("a push older than a quarter carries its date instead of a count", () => {
  assert.equal(pushed("2026-03-14T09:00:00Z", NOW), "pushed 2026-03-14");
});

test("a repository with nothing behind it says nothing", () => {
  assert.equal(pushed("", NOW), "");
  assert.equal(pushed("0001-01-01T00:00:00Z", NOW), "");
  assert.equal(pushed(undefined, NOW), "");
});
