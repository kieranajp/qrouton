import assert from "node:assert/strict";
import test from "node:test";
import { digestMeta } from "./digest.js";

const now = Date.parse("2026-10-11T15:00:00Z");

test("a digest keeps its lines and says how old it is", () => {
  assert.deepEqual(digestMeta({ text: "Plan written.\nWaiting on approval.\n", at: "2026-10-11T14:56:00Z" }, now), {
    text: "Plan written.\nWaiting on approval.",
    meta: "4m ago",
  });
});

test("a digest written this minute reads now, without ago", () => {
  assert.equal(digestMeta({ text: "Plan written.", at: "2026-10-11T14:59:40Z" }, now)?.meta, "just now");
});

test("no digest, or a blank one, shows nothing", () => {
  assert.equal(digestMeta(undefined, now), null);
  assert.equal(digestMeta({ text: "", at: "" }, now), null);
  assert.equal(digestMeta({ text: " \n ", at: "2026-10-11T14:56:00Z" }, now), null);
});
