import assert from "node:assert/strict";
import test from "node:test";
import { matchSessions, initialSelection, reconcileSelection } from "./switcher.js";
const row = (slug, name = slug, repos = []) => ({ slug, name, repos: repos.map((name) => ({ name })) });
const slugs = (rows, query) => matchSessions(rows, query).map(({ row }) => row.slug);
test("normalizes case and tokens across identity and every repository", () => {
  assert.deepEqual(slugs([row("one", "Session Switcher", ["web", "acme/backend"])], "  SWCH  bcknd "), ["one"]);
  assert.deepEqual(slugs([row("one", "Session Switcher")], "swch missing"), []);
});
test("exact, prefix, contiguous and subsequence classes outrank in order", () => {
  const rows = [row("a", "s-w-i-t-c-h"), row("b", "a switch"), row("c", "switcher"), row("d", "switch")];
  assert.deepEqual(slugs(rows, "switch"), ["d", "c", "b", "a"]);
});
test("ties prefer name then slug then repositories and retain supplied order", () => {
  assert.deepEqual(slugs([row("r", "other", ["foo"]), row("foo", "other"), row("n", "foo"), row("n2", "foo")], "foo"), ["n", "n2", "foo", "r"]);
});
test("empty query preserves order and selects another session", () => {
  const matches = matchSessions([row("a"), row("b")]);
  assert.deepEqual(matches.map(({ row }) => row.slug), ["a", "b"]);
  assert.equal(initialSelection(matches, "a"), "b");
  assert.equal(reconcileSelection(matches, "a"), "a");
  assert.equal(reconcileSelection(matches, "gone"), "a");
  assert.equal(initialSelection([], "a"), "");
});
test("duplicate identities and hidden repository matches supply context", () => {
  assert.equal(matchSessions([row("a", "same"), row("b", "same")])[1].context, "b");
  assert.equal(matchSessions([row("a", "same", ["first", "hidden"])], "hidden")[0].context, "hidden");
  assert.deepEqual(matchSessions([{ slug: "a", name: null, repos: null }], "zzz"), []);
});
