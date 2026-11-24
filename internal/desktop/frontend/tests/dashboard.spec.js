import { expect, test } from "@playwright/test";

test.use({ viewport: { width: 1280, height: 800 } });

test.beforeEach(async ({ page }) => {
  await page.goto("/tests/dashboard.html");
  await page.waitForFunction(() => Boolean(window.dashboard));
});

const card = (page, slug) => page.locator(`[data-card="${slug}"]`);
const columns = (page) =>
  page.locator(".column").evaluateAll((nodes) =>
    nodes.map((node) => [...node.querySelectorAll("[data-card]")].map((card) => card.getAttribute("data-card"))),
  );

// The titlebar names the dashboard; a heading here as well sat off the titlebar's centre by half the rail.
test("the header carries no heading, only the freshness line and Refresh at its end", async ({ page }) => {
  await expect(page.getByRole("region", { name: "Dashboard" })).toBeVisible();
  await expect(page.getByRole("heading")).toHaveCount(0);
  const edges = await page.locator(".dashboard header").evaluate((header) => ({
    header: header.getBoundingClientRect().right,
    refresh: header.querySelector("button").getBoundingClientRect().right,
    freshness: header.querySelector(".freshness").getBoundingClientRect().right,
    left: header.querySelector("button").getBoundingClientRect().left,
  }));
  expect(Math.abs(edges.header - edges.refresh)).toBeLessThan(1);
  expect(edges.freshness).toBeLessThanOrEqual(edges.left);
});

// Cards placed column by column would put ⌘2 under ⌘1 instead of beside it.
test("the first row reads left to right in rail order, each card hinting its rail shortcut", async ({ page }) => {
  const placed = await columns(page);
  expect(placed).toHaveLength(3);
  expect(placed.map((column) => column[0])).toEqual(["billing", "session-2", "session-3"]);
  await expect(card(page, "billing")).toContainText("⌘1");
  await expect(card(page, "session-3")).toContainText("⌘3");
  await expect(card(page, "session-9")).toContainText("⌘9");
  await expect(card(page, "session-10")).not.toContainText("⌘");
});

test("the grid holds one to four columns, set by its own width", async ({ page }) => {
  for (const [width, count] of [[870, 2], [400, 1], [1600, 4], [3000, 4], [1180, 3]]) {
    await page.evaluate((next) => window.dashboard.setWidth(next), width);
    await expect.poll(async () => (await columns(page)).length).toBe(count);
  }
});

// Opened times that agree with neither the new order nor its reverse, so only rail order passes.
test("cards follow a reordered rail", async ({ page }) => {
  await page.evaluate(() => {
    const rows = window.dashboard.sessions();
    const at = (hours) => new Date(Date.now() - hours * 3600000).toISOString();
    window.dashboard.setSessions([
      { ...rows[2], opened: at(5) },
      { ...rows[0], opened: at(1) },
      { ...rows[1], opened: at(9) },
    ]);
  });
  expect((await columns(page)).map((column) => column[0])).toEqual(["session-3", "billing", "session-2"]);
  await expect(card(page, "session-3")).toContainText("⌘1");
});

// Catches a card calling the bridge itself, where a refused open would go unreported.
test("a card head is one button that hands its slug to the shared opener", async ({ page }) => {
  const billing = card(page, "billing");
  await expect(billing.getByRole("button")).toHaveCount(1);
  await billing.getByRole("button").click();
  expect(await page.evaluate(() => window.dashboardBridge.opened)).toEqual(["billing"]);
  expect(await page.evaluate(() => window.dashboardBridge.calls())).toEqual([]);
});

// An RPI marker on the card would add noise to every guided session.
test("a head names its repositories and says when it was last opened, with no RPI marker", async ({ page }) => {
  const billing = card(page, "billing");
  const repos = billing.locator(".repos");
  await expect(repos).toHaveText("acme/web, acme/billing, acme/a-very-long-repository-name-that-overflows");
  expect(await repos.evaluate((node) => node.scrollWidth > node.clientWidth)).toBe(true);
  expect(await repos.evaluate((node) => getComputedStyle(node).textOverflow)).toBe("ellipsis");
  await expect(billing.locator(".head")).not.toContainText("RPI");
  await expect(billing.locator(".age")).toHaveText("3w");

  const plain = card(page, "session-2");
  await expect(plain.locator(".repos")).toHaveText("No editing repositories");
  await expect(plain.locator(".age")).toHaveText("2h");
});

test("a long title stops at two lines", async ({ page }) => {
  const title = card(page, "billing").locator(".title");
  const lines = await title.evaluate((node) => node.getBoundingClientRect().height / parseFloat(getComputedStyle(node).lineHeight));
  expect(Math.round(lines)).toBe(2);
});

const NEVER = "0001-01-01T00:00:00Z";
const freshness = (page) => page.locator(".freshness");
const refresh = (page) => page.getByRole("button", { name: "Refresh" });
const push = (page, extra) => page.evaluate((next) => window.pushDashboard(window.dashboardStatus(next)), extra);

// The cache keeps every pull request on the branch; a card that skipped the author filter would claim a colleague's.
test("another author's pull request on the same branch stays off the card", async ({ page }) => {
  const billing = card(page, "billing");
  await expect(billing.getByRole("link")).toHaveCount(2);
  await expect(billing).not.toContainText("Their pull request");
});

// A visible state word would repeat the icon on every row; a review tag gated on open would hide a merged pull request's approval.
test("a block leads with its state icon, named only to assistive tech, then its tags, repo, number and age on one line", async ({ page }) => {
  const [first, second] = await card(page, "billing").getByRole("link").all();
  await expect(first.getByRole("img", { name: "Open" })).toHaveCount(1);
  await expect(first.locator(".row")).not.toContainText("Open");
  await expect(first.locator(".chip")).toHaveText(["Changes requested", "Checks failing"]);
  await expect(first.locator(".repo")).toHaveText("acme/web#41");
  await expect(first.locator(".when")).toHaveText("41m");
  await expect(second.getByRole("img", { name: "Merged" })).toHaveCount(1);
  await expect(second.locator(".row")).not.toContainText("Merged");
  await expect(second.locator(".chip")).toHaveText(["Approved"]);
  expect(await second.locator(".state").evaluate((node) => getComputedStyle(node).color)).toBe("rgb(198, 160, 246)");
  const row = await first.locator(".row").evaluate((node) => ({ height: node.getBoundingClientRect().height, line: parseFloat(getComputedStyle(node).lineHeight) }));
  expect(row.height).toBeLessThan(row.line * 2);
  const lines = await first.locator(".headline").evaluate((node) => node.getBoundingClientRect().height / parseFloat(getComputedStyle(node).lineHeight));
  expect(Math.round(lines)).toBe(2);
});

// Summing the reference checkout would credit the session with commits it never made.
test("the git line sums the editing repositories and a session with none committed says so", async ({ page }) => {
  await expect(card(page, "billing").locator(".git")).toHaveText("5 commits · +611 −204");
  await expect(card(page, "session-3").locator(".git")).toHaveText("no commits");
});

// Expansion held in the card would fold back the moment the dashboard unmounts behind a session.
test("four pull requests show, then Show 3 more expands in place and stays expanded after leaving", async ({ page }) => {
  const busy = card(page, "session-3");
  await expect(busy.getByRole("link")).toHaveCount(4);
  await busy.getByRole("button", { name: "Show 3 more pull requests in Session 3" }).click();
  await expect(busy.getByRole("link")).toHaveCount(7);
  await expect(busy.getByRole("button", { name: /Show \d+ more/ })).toHaveCount(0);

  await page.evaluate(() => window.dashboard.setShown(false));
  await expect(page.locator("[data-card]")).toHaveCount(0);
  await page.evaluate(() => window.dashboard.setShown(true));
  await expect(card(page, "session-3").getByRole("link")).toHaveCount(7);
});

// Focus left on a button that vanished would drop a keyboard user back at the top of the page.
test("expanding moves focus to the first pull request it revealed", async ({ page }) => {
  const busy = card(page, "session-3");
  await busy.getByRole("button", { name: /Show 3 more/ }).focus();
  await page.keyboard.press("Enter");
  await expect(busy.getByRole("link").nth(4)).toBeFocused();
});

// Re-estimating an expanded card would reflow every card after it into other columns.
test("expanding a card leaves every other card in its column", async ({ page }) => {
  const before = await columns(page);
  await card(page, "session-3").getByRole("button", { name: /Show 3 more/ }).click();
  await expect(card(page, "session-3").getByRole("link")).toHaveCount(7);
  expect(await columns(page)).toEqual(before);
});

// A link nested in the head, or a head that swallowed the link's click, would open the wrong thing.
test("a block's link and the card head are separate targets", async ({ page }) => {
  const billing = card(page, "billing");
  await expect(billing.locator("button a, a button")).toHaveCount(0);
  await billing.getByRole("link").first().click();
  expect(await page.evaluate(() => window.openedURL)).toBe("https://github.com/acme/web/pull/41");
  expect(await page.evaluate(() => window.dashboardBridge.opened)).toEqual([]);
  await billing.locator(".head").click();
  expect(await page.evaluate(() => window.dashboardBridge.opened)).toEqual(["billing"]);
});

const state = (pullState, pullRequests = []) => ({ pullRequests, pullState, repos: [] });

// Judging cards by the whole refresh let one session's answer mark another as empty, and left a card with no branch checking for good.
test("each card says whether its own branches wait, answered, failed or have none to ask about", async ({ page }) => {
  await push(page, { checkedAt: NEVER, viewer: "", sessions: { billing: state("pending"), "session-2": state("none") } });
  await expect(card(page, "billing").locator(".note")).toHaveText("Checking GitHub…");
  await expect(card(page, "session-2").locator(".note")).toHaveText("No branch to check");
  await push(page, { checkedAt: NEVER, viewer: "", signedOut: true, sessions: { billing: state("pending"), "session-2": state("none") } });
  await expect(card(page, "billing").locator(".note")).toHaveCount(0);
  await expect(card(page, "session-2").locator(".note")).toHaveText("No branch to check");
  await push(page, { checkedAt: new Date().toISOString(), sessions: { billing: state("answered"), "session-2": state("unavailable") } });
  await expect(card(page, "billing").locator(".note")).toHaveText("No pull requests");
  await expect(card(page, "session-2").locator(".note")).toHaveText("Couldn't load pull requests");
});

// A card with one branch failed must keep the rows it has and still say the rest did not load.
test("a card with rows from one branch and another failed shows both", async ({ page }) => {
  await push(page, {
    sessions: { billing: state("unavailable", [{ repo: "acme/web", number: 41, title: "Kept", url: "https://github.com/acme/web/pull/41", state: "open", author: "octocat" }]) },
  });
  await expect(card(page, "billing").locator("a")).toHaveCount(1);
  await expect(card(page, "billing").locator(".note")).toHaveText("Couldn't load every repository");
});

// A Refresh that stayed live mid-refresh would queue repeat requests and give no sign anything was happening.
test("Refresh asks Go to refresh, then spins and disables while it runs", async ({ page }) => {
  await refresh(page).click();
  expect(await page.evaluate(() => window.dashboardBridge.calls().map((call) => call.method))).toEqual(["Refresh"]);
  await push(page, { refreshing: true });
  await expect(refresh(page)).toBeDisabled();
  await expect(refresh(page)).toHaveAttribute("aria-busy", "true");
  expect(await refresh(page).locator(".icon").evaluate((node) => getComputedStyle(node).animationName)).not.toBe("none");
  await push(page, {});
  await expect(refresh(page)).toBeEnabled();
});

test.describe("the header line", () => {
  // Noon in a zone half an hour off UTC, so the expected times below are fixed and a UTC clock cannot match them.
  const NOON = Date.parse("2026-11-20T06:30:00Z");
  test.use({ timezoneId: "Asia/Kolkata" });
  test.beforeEach(async ({ page }) => {
    await page.clock.install({ time: NOON });
    await page.reload();
    await page.waitForFunction(() => Boolean(window.dashboard));
  });

  // A header that said "just now" for any age, or dropped the day of an overnight cache, would pass old rows off as fresh.
  test("the header says when the rows are from, with the date when it is not today", async ({ page }) => {
    await push(page, { checkedAt: "2026-11-20T06:20:00Z" });
    await expect(freshness(page)).toHaveText("As of 11:50");
    await push(page, { checkedAt: "2026-11-18T12:00:00Z" });
    await expect(freshness(page)).toHaveText("As of 18 Nov 17:30");
    await push(page, { checkedAt: "2026-11-20T06:30:00Z" });
    await expect(freshness(page)).toHaveText("Updated just now");
  });

  // A header computed only when an event arrives would say "just now" all morning.
  test("the header ages on its own as time passes", async ({ page }) => {
    await push(page, { checkedAt: "2026-11-20T06:30:00Z" });
    await expect(freshness(page)).toHaveText("Updated just now");
    await page.clock.fastForward("02:00");
    await expect(freshness(page)).toHaveText("As of 12:00");
  });

  // A failure drawn like freshness would pass old rows off as current.
  test("a failed refresh reads in the warning colour, with the last good time when there is one", async ({ page }) => {
    await push(page, { failed: true, checkedAt: "2026-11-20T06:20:00Z" });
    await expect(freshness(page)).toHaveText("Couldn't reach GitHub · showing 11:50");
    expect(await freshness(page).evaluate((node) => getComputedStyle(node).color)).toBe("rgb(245, 169, 127)");
    await push(page, { failed: true, checkedAt: NEVER });
    await expect(freshness(page)).toHaveText("Couldn't reach GitHub");
  });

  // An enabled Refresh during a limit would spend requests GitHub has said it will refuse.
  test("a rate limit names the next try and disables Refresh until then", async ({ page }) => {
    await expect(refresh(page)).toBeEnabled();
    await push(page, { pausedUntil: "2026-11-20T06:42:00Z" });
    await expect(freshness(page)).toHaveText("GitHub rate limit · next try 12:12");
    await expect(refresh(page)).toBeDisabled();
  });

  // A failure line in its place, or no way to the settings, would hide the one fix the user can make.
  test("signed out, the header offers the GitHub settings tab and outranks a failure", async ({ page }) => {
    await push(page, { signedOut: true, failed: true });
    await expect(freshness(page)).toContainText("GitHub is not signed in");
    await expect(card(page, "billing").getByRole("link")).toHaveCount(2);
    await freshness(page).getByRole("button", { name: "GitHub settings" }).click();
    expect(await page.evaluate(() => window.dashboardBridge.settings)).toEqual(["github"]);
  });
});

// Catches a card that hides its todos, drops the blocking count, or marks the wrong item as blocking.
test("a card with open todos counts them, then lists its blocking and next items one line each", async ({ page }) => {
  const todos = card(page, "session-2").locator(".todos");
  await expect(todos.locator(".tally")).toHaveText("3 open · 1 blocking");
  await expect(todos.getByRole("button")).toHaveCount(2);
  await expect(todos.locator(".todo .text")).toHaveText(["Apply the migration", "Announce in the channel"]);
  await expect(todos.locator(".todo").nth(0).locator(".blocks")).toHaveText("Blocking");
  await expect(todos.locator(".todo").nth(1).locator(".blocks")).toHaveCount(0);
  await expect(card(page, "session-3").locator(".todos")).toHaveCount(0);
});

// Catches opening the document before the session shows, or without its session, path or line.
test("a card's todo shows its session, then opens the document at the item's line", async ({ page }) => {
  await card(page, "session-2").getByRole("button", { name: /Announce in the channel/ }).click();
  await expect
    .poll(() => page.evaluate(() => window.dashboardBridge.calls().map((call) => [call.method, ...call.args])))
    .toEqual([
      ["Show", "session-2"],
      ["OpenDocumentAt", "session-2", "thoughts/shared/todos/T1-rollout.md", 22],
    ]);
});

// Catches a refused show still opening the document, which would land in whichever session is on screen.
test("a card's todo opens nothing when its session will not show", async ({ page }) => {
  await page.evaluate(() => window.dashboardBridge.refuseNextShow());
  await card(page, "session-2").getByRole("button", { name: /Apply the migration/ }).click();
  await expect.poll(() => page.evaluate(() => window.dashboardBridge.calls().map((call) => call.method))).toEqual(["Show"]);
  await page.waitForTimeout(100);
  expect(await page.evaluate(() => window.dashboardBridge.calls().map((call) => call.method))).toEqual(["Show"]);
});
