import assert from "node:assert/strict";
import test from "node:test";
import { DASHBOARD_KEY, matchSessions, initialSelection, preferredSelection, reconcileSelection, shownKey } from "./switcher.js";
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
test("empty query leads with the Dashboard, preserves order and selects another session", () => {
  const matches = matchSessions([row("a"), row("b")]);
  assert.deepEqual(matches.map(({ row }) => row.slug), [DASHBOARD_KEY, "a", "b"]);
  assert.equal(initialSelection(matches, "a"), "b");
  assert.equal(reconcileSelection(matches, "a"), "a");
  assert.equal(reconcileSelection(matches, "gone"), "a");
  assert.equal(initialSelection([], "a"), "");
});

// Selecting the leading Dashboard entry on open would turn ⌘K then Enter into "go to the dashboard".
test("opening on the dashboard or a session still selects a session first", () => {
  const matches = matchSessions([row("a"), row("b")]);
  assert.equal(initialSelection(matches, shownKey("")), "a");
  assert.equal(initialSelection(matches, shownKey("a")), "b");
  assert.equal(initialSelection(matchSessions([row("a")]), "a"), "a");
  assert.equal(initialSelection(matchSessions([]), shownKey("")), DASHBOARD_KEY);
});

// Catches ⌘K then Enter from the dashboard landing on the top rail row instead of the session just left.
test("opening on the dashboard selects the session opened last, whatever its rail position", () => {
  const rail = [{ ...row("a"), opened: "2026-11-20T08:00:00Z" }, { ...row("b"), opened: "2026-11-20T09:00:00Z" }];
  assert.equal(initialSelection(matchSessions(rail), shownKey("")), "b");
  assert.deepEqual(matchSessions(rail).map(({ row }) => row.slug), [DASHBOARD_KEY, "a", "b"]);
  assert.equal(initialSelection(matchSessions(rail), "b"), "a");
  assert.equal(initialSelection(matchSessions([{ ...row("a"), opened: "0001-01-01T00:00:00Z" }, row("b")]), shownKey("")), "a");
});

test("the Dashboard entry matches its name and nothing a session carries", () => {
  const rows = [row("billing", "Billing", ["acme/dash-api"]), row("web")];
  assert.deepEqual(slugs(rows, "dashboard"), [DASHBOARD_KEY]);
  assert.deepEqual(slugs(rows, "dash"), [DASHBOARD_KEY, "billing"]);
  assert.deepEqual(slugs(rows, "web"), ["web"]);
});

// Taking the leader for an empty query would select the Dashboard whenever the query is cleared.
test("a query selects its best match, and clearing it goes back to the first session", () => {
  const rows = [row("a"), row("b")];
  assert.equal(preferredSelection(matchSessions(rows, "dashboard"), "dashboard"), DASHBOARD_KEY);
  assert.equal(preferredSelection(matchSessions(rows, "b"), "b"), "b");
  assert.equal(preferredSelection(matchSessions(rows, ""), ""), "a");
  assert.equal(reconcileSelection(matchSessions(rows, "dash"), "gone", "dash"), DASHBOARD_KEY);
});

// Matched on its key as well as its name, the entry would surface for a query no user would type for it.
test("the Dashboard entry is not found by its internal key", () => {
  assert.deepEqual(slugs([row("a")], ":"), []);
  assert.deepEqual(slugs([row("a")], DASHBOARD_KEY), []);
});

test("the dashboard is what is shown when no session is", () => {
  assert.equal(shownKey(""), DASHBOARD_KEY);
  assert.equal(shownKey("billing"), "billing");
});
test("duplicate identities and hidden repository matches supply context", () => {
  assert.equal(matchSessions([row("a", "same"), row("b", "same")])[2].context, "b");
  assert.equal(matchSessions([row("a", "same", ["first", "hidden"])], "hidden")[0].context, "hidden");
  assert.deepEqual(matchSessions([{ slug: "a", name: null, repos: null }], "zzz"), []);
});
