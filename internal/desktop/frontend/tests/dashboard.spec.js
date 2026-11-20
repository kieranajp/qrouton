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

test("the title is centred over the grid in the display face", async ({ page }) => {
  const title = page.getByRole("heading", { name: "Dashboard", level: 1 });
  await expect(title).toBeVisible();
  expect(await title.evaluate((node) => getComputedStyle(node).fontFamily)).toContain("Space Grotesk");
  const centres = await page.locator(".dashboard header").evaluate((header) => {
    const outer = header.getBoundingClientRect();
    const inner = header.querySelector("h1").getBoundingClientRect();
    return { header: outer.left + outer.width / 2, title: inner.left + inner.width / 2 };
  });
  expect(Math.abs(centres.header - centres.title)).toBeLessThan(1);
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

test("a card head is one button that opens its session", async ({ page }) => {
  const billing = card(page, "billing");
  await expect(billing.getByRole("button")).toHaveCount(1);
  await billing.getByRole("button").click();
  expect(await page.evaluate(() => window.dashboardBridge.calls())).toEqual([{ method: "Show", args: ["billing"] }]);
});

test("a head names its repositories, tags a guided session and says when it was last opened", async ({ page }) => {
  const billing = card(page, "billing");
  const repos = billing.locator(".repos");
  await expect(repos).toHaveText("acme/web, acme/billing, acme/a-very-long-repository-name-that-overflows");
  expect(await repos.evaluate((node) => node.scrollWidth > node.clientWidth)).toBe(true);
  expect(await repos.evaluate((node) => getComputedStyle(node).textOverflow)).toBe("ellipsis");
  await expect(billing.locator(".tag")).toHaveText("RPI");
  await expect(billing.locator(".age")).toHaveText("3w");

  const plain = card(page, "session-2");
  await expect(plain.locator(".tag")).toHaveCount(0);
  await expect(plain.locator(".repos")).toHaveText("No editing repositories");
  await expect(plain.locator(".age")).toHaveText("2h");
});

test("a long title stops at two lines", async ({ page }) => {
  const title = card(page, "billing").locator(".title");
  const lines = await title.evaluate((node) => node.getBoundingClientRect().height / parseFloat(getComputedStyle(node).lineHeight));
  expect(Math.round(lines)).toBe(2);
});
