import assert from "node:assert/strict";
import test from "node:test";
import { SIGNED_OUT } from "../overview/pulls.js";
import {
  CHECKING,
  NO_PULLS,
  NO_SOURCE,
  PARTIAL,
  UNAVAILABLE,
  cardHeight,
  columnCount,
  estimatedHeight,
  freshness,
  gitLine,
  masonry,
  ownPulls,
  pullSection,
  pullTags,
  refreshDisabled,
  repoList,
  showMore,
  showMoreLabel,
  todoItems,
  todoLine,
} from "./dashboard.js";

process.env.TZ = "Asia/Kolkata";

test("the design's two window sizes get the columns it drew", () => {
  assert.equal(columnCount(1120), 3);
  assert.equal(columnCount(810), 2);
});

test("columns stay between one and four however wide or narrow the grid is", () => {
  assert.equal(columnCount(0), 1);
  assert.equal(columnCount(200), 1);
  assert.equal(columnCount(5000), 4);
  assert.equal(columnCount(NaN), 1);
});

// A count that ignored the UI scale would squeeze enlarged cards into columns too narrow for them.
test("a larger UI scale fits fewer columns into the same width", () => {
  assert.equal(columnCount(1120, 1), 3);
  assert.equal(columnCount(1120, 1.5), 2);
});

test("a card's estimate grows by one block per pull request it shows", () => {
  assert.equal(estimatedHeight(), 100);
  assert.equal(estimatedHeight(0), 100);
  assert.equal(estimatedHeight(3), 286);
});

// Placing by index alone would leave a tall card's column growing while its neighbours sat short.
test("each card goes to the column that is shortest when it is placed", () => {
  const heights = [estimatedHeight(4), estimatedHeight(0), estimatedHeight(0), estimatedHeight(0)];
  assert.deepEqual(masonry(heights, 2), [[0], [1, 2, 3]]);
});

// Ties broken anywhere but left would scramble the rail order across the first row.
test("equal cards fill the first row left to right, in rail order", () => {
  assert.deepEqual(masonry([100, 100, 100, 100, 100], 3), [[0, 3], [1, 4], [2]]);
});

test("one column holds every card in order, and no cards leave every column empty", () => {
  assert.deepEqual(masonry([100, 200, 100], 1), [[0, 1, 2]]);
  assert.deepEqual(masonry([], 3), [[], [], []]);
  assert.deepEqual(masonry([100], 0), [[0]]);
});

test("the repos line names every repository, and none says so", () => {
  assert.equal(repoList([{ name: "acme/web" }, { name: "acme/api" }]), "acme/web, acme/api");
  assert.equal(repoList([]), "No editing repositories");
});

const NEVER = "0001-01-01T00:00:00Z";
const NOW = Date.parse("2026-11-20T09:00:00Z");
const minutesAgo = (minutes) => new Date(NOW - minutes * 60000).toISOString();
const pull = (number, extra = {}) => ({
  repo: "acme/web",
  number,
  title: `Pull ${number}`,
  url: `https://github.com/acme/web/pull/${number}`,
  state: "open",
  author: "octocat",
  review: "",
  checksFailing: false,
  updatedAt: minutesAgo(number),
  ...extra,
});
const fresh = (extra = {}) => ({ viewer: "octocat", checkedAt: minutesAgo(10), refreshing: false, failed: false, signedOut: false, pausedUntil: NEVER, sessions: {}, ...extra });

// The cache keeps every pull request on the branch, so a missing filter shows a colleague's work as the user's.
test("only the viewer's own pull requests are kept, and none before GitHub has named the viewer", () => {
  const pulls = [pull(1), pull(2, { author: "someone-else" }), pull(3, { author: "" })];
  assert.deepEqual(ownPulls(pulls, "octocat").map((pr) => pr.number), [1]);
  assert.deepEqual(ownPulls(pulls, ""), [], "a deleted account's pull request is nobody's, not an unnamed viewer's");
  assert.deepEqual(ownPulls(null, "octocat"), []);
});

// Sorting by update alone would bury an open pull request under yesterday's merges.
test("open and draft come before merged and closed, each newest update first", () => {
  const pulls = [
    pull(1, { state: "merged" }),
    pull(5, { state: "open" }),
    pull(2, { state: "closed" }),
    pull(3, { state: "draft" }),
    pull(9, { state: "merged" }),
  ];
  assert.deepEqual(ownPulls(pulls, "octocat").map((pr) => pr.number), [3, 5, 1, 2, 9]);
});

// Showing all seven, or losing the count, would either flood the card or hide work without saying so.
test("a card shows four pull requests and counts the rest until it is expanded", () => {
  const status = fresh({ sessions: { billing: { pullRequests: Array.from({ length: 7 }, (_, i) => pull(i + 1)), repos: [] } } });
  const folded = pullSection(status, "billing", NOW);
  assert.deepEqual(folded.shown.map((pr) => pr.number), [1, 2, 3, 4]);
  assert.equal(folded.more, 3);
  assert.equal(showMore(folded.more), "Show 3 more");
  assert.equal(showMoreLabel(folded.more, "Billing"), "Show 3 more pull requests in Billing");
  const open = pullSection(status, "billing", NOW, { expanded: true });
  assert.equal(open.shown.length, 7);
  assert.equal(open.more, 0);
});

// Growing the estimate on expand would reflow every card after it into other columns.
test("a card's height estimate counts at most four pull requests, expanded or not", () => {
  const status = fresh({ sessions: { billing: { pullRequests: Array.from({ length: 7 }, (_, i) => pull(i + 1)), repos: [] } } });
  const folded = cardHeight(pullSection(status, "billing", NOW));
  assert.equal(folded, estimatedHeight(4));
  assert.equal(cardHeight(pullSection(status, "billing", NOW, { expanded: true })), folded);
  assert.equal(cardHeight({ shown: [pull(1)], more: 0 }), estimatedHeight(1));
});

// Waiting on a card GitHub has not answered would leave it saying it is checking for good once the header explains why.
test("a card checks GitHub only while its own branches wait and nothing has said why", () => {
  const empty = { checkedAt: NEVER, viewer: "", sessions: { billing: { pullRequests: [], pullState: "pending", repos: [] } } };
  assert.equal(pullSection(empty, "billing", NOW).note, CHECKING);
  assert.equal(pullSection({ ...empty, signedOut: true }, "billing", NOW).note, "");
  assert.equal(pullSection({ ...empty, failed: true }, "billing", NOW).note, "");
  assert.equal(pullSection({ ...empty, pausedUntil: minutesAgo(-5) }, "billing", NOW).note, "");
  assert.equal(pullSection({ ...empty, pausedUntil: minutesAgo(5) }, "billing", NOW).note, CHECKING, "a pause that has passed is waiting on GitHub again");
  assert.equal(pullSection(fresh({ sessions: {} }), "billing", NOW).note, CHECKING, "a session the payload has not reached yet is still waiting");
});

const card = (pullState, pullRequests = []) => ({ pullRequests, pullState, repos: [] });

// Reading the global checkedAt let one session's answer mark another, whose only branch failed, as empty.
test("on an empty cache, one answered session leaves another whose branch failed unavailable", () => {
  const status = fresh({ checkedAt: minutesAgo(0), sessions: { web: card("answered"), api: card("unavailable") } });
  assert.equal(pullSection(status, "web", NOW).note, NO_PULLS);
  assert.equal(pullSection(status, "api", NOW).note, UNAVAILABLE);
});

// A session with no branch to ask about never gets an answer, so waiting on one would say Checking GitHub for good.
test("a session with no branch to ask about says so at once, whatever the header says", () => {
  for (const extra of [{ checkedAt: NEVER, viewer: "" }, {}, { signedOut: true }, { failed: true }]) {
    assert.equal(pullSection(fresh({ ...extra, sessions: { docs: card("none") } }), "docs", NOW).note, NO_SOURCE);
  }
});

// Hiding an unavailable card behind the header would leave a failed branch looking like one still being checked.
test("an unavailable card says so even while the header reports the failure", () => {
  assert.equal(pullSection(fresh({ failed: true, sessions: { api: card("unavailable") } }), "api", NOW).note, UNAVAILABLE);
});

// A mixed card that read as empty, or dropped its rows, would hide either the failure or the work.
test("a card with one branch answered and another failed shows its rows and says not every repository loaded", () => {
  const status = fresh({ sessions: { billing: card("unavailable", [pull(1)]) } });
  const section = pullSection(status, "billing", NOW);
  assert.deepEqual(section.shown.map((pr) => pr.number), [1]);
  assert.equal(section.note, PARTIAL);
  assert.equal(pullSection(fresh({ sessions: { billing: card("answered", [pull(1)]) } }), "billing", NOW).note, "");
});

// Gating the review on open hid the approval a merged pull request landed with.
test("every state keeps its review tag, checks failing marks only open and draft, and both may sit side by side", () => {
  const labels = (pr) => pullTags(pr).map((tag) => `${tag.label}:${tag.tone}`);
  assert.deepEqual(labels(pull(1, { review: "approved" })), ["Approved:success"]);
  assert.deepEqual(labels(pull(1, { state: "draft", review: "changes-requested" })), ["Changes requested:waiting"]);
  assert.deepEqual(labels(pull(1, { review: "approved", checksFailing: true })), ["Approved:success", "Checks failing:failed"]);
  assert.deepEqual(labels(pull(1, { state: "draft", checksFailing: true })), ["Checks failing:failed"]);
  assert.deepEqual(labels(pull(1, { state: "merged", review: "approved", checksFailing: true })), ["Approved:success"]);
  assert.deepEqual(labels(pull(1, { state: "closed", review: "changes-requested" })), ["Changes requested:waiting"]);
  assert.deepEqual(labels(pull(1, { state: "closed", checksFailing: true })), []);
  assert.deepEqual(labels(pull(1)), []);
});

// Counting a reference checkout would add commits the session never made.
test("the git line sums the measured editing repositories", () => {
  const repo = (extra) => ({ name: "acme/web", role: "editing", measured: true, commits: 0, insertions: 0, deletions: 0, ...extra });
  assert.equal(
    gitLine([
      repo({ commits: 3, insertions: 600, deletions: 200 }),
      repo({ commits: 2, insertions: 11, deletions: 4 }),
      repo({ role: "reference", commits: 40, insertions: 9, deletions: 9 }),
      repo({ measured: false, commits: 8 }),
    ]),
    "5 commits · +611 −204",
  );
  assert.equal(gitLine([repo({ commits: 1, insertions: 2, deletions: 0 })]), "1 commit · +2 −0");
  assert.equal(gitLine([repo({})]), "no commits");
  assert.equal(gitLine([repo({ measured: false })]), "");
  assert.equal(gitLine(null), "");
});

// A header that said "just now" for any age, or nothing once checked, would hide stale rows.
test("the header says how old the rows are", () => {
  assert.deepEqual(freshness(fresh({ checkedAt: minutesAgo(0.5) }), NOW), { text: "Updated just now", warn: false, settings: false });
  assert.deepEqual(freshness(fresh(), NOW), { text: "As of 14:20", warn: false, settings: false });
  assert.equal(freshness(fresh({ checkedAt: NEVER }), NOW).text, "");
});

// A cache kept overnight would read as this morning's without its date.
test("rows from an earlier day carry their date in every header line", () => {
  const yesterday = "2026-11-18T12:00:00Z";
  assert.equal(freshness(fresh({ checkedAt: yesterday }), NOW).text, "As of 18 Nov 17:30");
  assert.equal(freshness(fresh({ checkedAt: yesterday, failed: true }), NOW).text, "Couldn't reach GitHub · showing 18 Nov 17:30");
  assert.equal(freshness(fresh({ pausedUntil: "2026-11-20T19:00:00Z" }), NOW).text, "GitHub rate limit · next try 21 Nov 00:30");
});

// A failure drawn as freshness would pass off old rows as current.
test("a failed refresh says so in the warning tone, with the last good time when there is one", () => {
  assert.deepEqual(freshness(fresh({ failed: true }), NOW), { text: "Couldn't reach GitHub · showing 14:20", warn: true, settings: false });
  assert.equal(freshness(fresh({ failed: true, checkedAt: NEVER }), NOW).text, "Couldn't reach GitHub");
});

// A limit that has passed must stop disabling Refresh, or the user waits for an event that may never come.
test("a rate limit names its next try only until that time passes", () => {
  const limited = fresh({ failed: true, pausedUntil: minutesAgo(-12) });
  assert.deepEqual(freshness(limited, NOW), { text: "GitHub rate limit · next try 14:42", warn: true, settings: false });
  assert.equal(refreshDisabled(limited, NOW), true);
  const passed = fresh({ pausedUntil: minutesAgo(1) });
  assert.equal(freshness(passed, NOW).text, "As of 14:20");
  assert.equal(refreshDisabled(passed, NOW), false);
});

// A rate limit or failure line in its place would hide the one fix the user can make.
test("signed out outranks every other state and offers the settings", () => {
  const out = fresh({ signedOut: true, failed: true, pausedUntil: minutesAgo(-12) });
  assert.deepEqual(freshness(out, NOW), { text: SIGNED_OUT, warn: false, settings: true });
});

// A live Refresh mid-refresh would queue one more request per click.
test("Refresh is disabled while a refresh runs", () => {
  assert.equal(refreshDisabled(fresh({ refreshing: true }), NOW), true);
  assert.equal(refreshDisabled(fresh(), NOW), false);
});

const todo = (text, line) => ({ text, section: "Ship", where: "", path: "thoughts/shared/todos/T1-rollout.md", line });

// Catches a todo line on a card whose todos are all done.
test("a card's todo line counts the open items", () => {
  assert.equal(todoLine({ open: 3, items: [] }), "3 open");
  assert.equal(todoLine({ open: 0, items: [] }), "");
  assert.equal(todoLine(undefined), "");
});

// Catches a card listing items the roll-up no longer counts as open, or a null list throwing.
test("a card lists the roll-up's items only while any are open", () => {
  const items = [todo("Migrate", 12), todo("Announce", 20)];
  assert.deepEqual(todoItems({ open: 2, items }), items);
  assert.deepEqual(todoItems({ open: 0, items }), []);
  assert.deepEqual(todoItems({ open: 1, items: null }), []);
});

// Catches masonry placing cards by their pull requests alone, so a column of todo-heavy cards ran long.
test("a card's estimate grows with its todo line and each item it lists", () => {
  const section = { shown: [], more: 0 };
  const bare = cardHeight(section);
  assert.equal(cardHeight(section, { open: 0, items: [] }), bare);
  const one = cardHeight(section, { open: 1, items: [todo("Announce", 20)] });
  const two = cardHeight(section, { open: 2, items: [todo("Migrate", 12), todo("Announce", 20)] });
  assert.ok(one > bare);
  assert.equal(two - one, (one - bare) / 2);
});
