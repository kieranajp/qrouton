import { expect, test } from "@playwright/test";

const review = (page) => page.getByRole("button", { name: "Review bug report", exact: true });
const create = (page) => page.getByRole("button", { name: "Create issue", exact: true });
const confirms = (page) => page.evaluate(() => window.bugs.calls("BugReports.Confirm"));
const cancels = (page) => page.evaluate(() => window.bugs.calls("BugReports.Cancel"));
const terminal = (page) => page.locator('.agent .host:visible textarea');

test.beforeEach(async ({ page }) => { await page.goto("/tests/bug-report.html"); });

test("arrival leaves typing and background session selection alone", async ({ page }) => {
  await expect(terminal(page)).toBeFocused();
  await page.keyboard.type("before");
  await page.evaluate(() => window.bugs.queue("second", "background"));
  await expect(review(page)).toHaveCount(0);
  await expect(terminal(page)).toBeFocused();
  await page.evaluate(() => window.bugs.queue());
  await expect(review(page)).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(terminal(page)).toBeFocused();
  await page.keyboard.type("after");
  expect(await confirms(page)).toEqual([]);
  expect((await page.evaluate(() => window.bugs.calls("Term.Write"))).length).toBeGreaterThan(0);
});

test("full escaped preview is scrollable at narrow width and fetches no images", async ({ page }) => {
  const remote = [];
  page.on("request", (request) => { if (request.url().includes("report.invalid")) remote.push(request.url()); });
  await page.evaluate(() => window.bugs.queue());
  await review(page).click();
  await page.setViewportSize({ width: 440, height: 660 });
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("kieranajp/qrouton");
  await expect(dialog).toContainText("A complete bug title one");
  await expect(dialog.locator("pre")).toContainText('<img src="https://report.invalid/track">');
  await expect(dialog.locator("img")).toHaveCount(0);
  expect(await dialog.locator(".payload").evaluate((el) => el.scrollHeight > el.clientHeight)).toBe(true);
  await dialog.locator(".payload").evaluate((el) => { el.scrollTop = el.scrollHeight; });
  await expect(dialog.locator("pre")).toContainText("final evidence");
  const bounds = await dialog.boundingBox();
  expect(bounds.x).toBeGreaterThanOrEqual(16);
  expect(bounds.y).toBeGreaterThanOrEqual(16);
  expect(bounds.x + bounds.width).toBeLessThanOrEqual(424);
  expect(bounds.y + bounds.height).toBeLessThanOrEqual(644);
  expect(remote).toEqual([]);
  expect(await confirms(page)).toEqual([]);
});

for (const action of ["Cancel", "Escape"]) {
  test(`${action} sends no confirm and restores conversation focus`, async ({ page }) => {
    await page.evaluate(() => window.bugs.queue());
    await review(page).click();
    await expect(create(page)).toBeEnabled();
    if (action === "Escape") await page.keyboard.press("Escape");
    else await page.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(terminal(page)).toBeFocused();
    expect(await confirms(page)).toEqual([]);
    expect(await cancels(page)).toEqual([["first", "one"]]);
  });
}

test("double activation submits once, and closing while posting retains the result", async ({ page }) => {
  await page.evaluate(() => window.bugs.queue());
  await review(page).click();
  await create(page).evaluate((el) => { el.click(); el.click(); });
  await expect.poll(() => confirms(page)).toEqual([["first", "one"]]);
  await expect(page.getByRole("status")).toContainText("Creating");
  await page.keyboard.press("Escape");
  expect(await cancels(page)).toEqual([]);
  await page.evaluate(() => window.bugs.finish("created"));
  await review(page).click();
  const link = page.getByRole("link", { name: "https://github.com/kieranajp/qrouton/issues/42" });
  await expect(link).toBeVisible();
  await link.click();
  expect(await page.evaluate(() => window.openedURL)).toBe("https://github.com/kieranajp/qrouton/issues/42");
});

test("keyboard review does not approve until Create issue is activated", async ({ page }) => {
  await page.evaluate(() => window.bugs.queue());
  await review(page).focus();
  await page.keyboard.press("Enter");
  await expect(create(page)).toBeEnabled();
  expect(await confirms(page)).toEqual([]);
  await create(page).focus();
  await page.keyboard.press("Enter");
  await expect.poll(() => confirms(page)).toEqual([["first", "one"]]);
});

test("session switching discards late preview replies and stale confirmation", async ({ page }) => {
  await page.evaluate(() => { window.bugs.queue(); window.bugs.delay(true); });
  await review(page).click();
  await expect.poll(() => page.evaluate(() => window.bugs.calls("BugReports.Load").length)).toBe(1);
  await page.evaluate(() => { window.bugs.select("second"); window.bugs.queue("second", "two"); window.bugs.delay(false); });
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await review(page).click();
  await expect(page.getByRole("dialog")).toContainText("A complete bug title two");
  await page.evaluate(() => window.bugs.release());
  await expect(page.getByRole("dialog")).not.toContainText("A complete bug title one");
  await create(page).click();
  expect(await confirms(page)).toEqual([["second", "two"]]);
});

for (const state of ["expired", "failed", "unknown"]) {
  test(`renders the ${state} outcome without allowing a second submission`, async ({ page }) => {
    await page.evaluate(() => window.bugs.queue());
    await review(page).click();
    await expect(create(page)).toBeEnabled();
    await page.evaluate((state) => window.bugs.finish(state), state);
    await expect(create(page)).toHaveCount(0);
    const expected = state === "expired" ? "No issue was sent" : state === "failed" ? "No GitHub token" : "inspect GitHub before retrying";
    await expect(page.getByRole("status")).toContainText(expected);
    expect(await confirms(page)).toEqual([]);
  });
}

test("a late confirm reply cannot replace a created outcome", async ({ page }) => {
  await page.evaluate(() => { window.bugs.queue(); window.bugs.delayConfirm(); });
  await review(page).click();
  await create(page).click();
  await expect(page.getByRole("status")).toContainText("Creating");
  await page.evaluate(() => window.bugs.finish("created"));
  await expect(page.getByRole("status")).toContainText("Created GitHub issue #42");
  await page.evaluate(() => window.bugs.releaseConfirm());
  await expect(page.getByRole("status")).toContainText("Created GitHub issue #42");
  expect(await confirms(page)).toEqual([["first", "one"]]);
});

test("a late cancel reply cannot dismiss another session's preview", async ({ page }) => {
  await page.evaluate(() => { window.bugs.queue(); window.bugs.delayCancel(); });
  await review(page).click();
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await page.evaluate(() => { window.bugs.select("second"); window.bugs.queue("second", "two"); });
  await review(page).click();
  await expect(page.getByRole("dialog")).toContainText("A complete bug title two");
  await page.evaluate(() => window.bugs.releaseCancel());
  await expect(create(page)).toBeEnabled();
  expect(await cancels(page)).toEqual([["first", "one"]]);
});

test("a lost confirmation reply reports uncertainty and disables another submission", async ({ page }) => {
  await page.evaluate(() => { window.bugs.queue(); window.bugs.failConfirm(); });
  await review(page).click();
  await create(page).click();
  await expect(page.getByRole("status")).toContainText("inspect GitHub before retrying");
  await expect(create(page)).toHaveCount(0);
  expect(await confirms(page)).toEqual([["first", "one"]]);
});
