import { expect, test } from "@playwright/test";

const latest = (page) => page.evaluate(() => window.reports.at(-1));
const waitForReport = (page, count) =>
  page.waitForFunction((minimum) => window.reports.length >= minimum, count);

test.beforeEach(async ({ page }) => {
  await page.goto("/tests/viewport.html");
  await waitForReport(page, 1);
});

test("partially intersecting blocks report their complete source intervals", async ({ page }) => {
  await page.locator("#root").evaluate((root) => (root.scrollTop = 100));
  await page.evaluate(() => window.measure());
  await page.waitForFunction(() => window.reports.at(-1)?.intervals?.[0]?.line === 3);
  await expect.poll(() => latest(page)).toEqual({
    seq: expect.any(Number),
    available: true,
    selected: true,
    intervals: [{ line: 3, to: 6 }],
  });
});

test("a viewport containing no stamped block is measured empty", async ({ page }) => {
  await page.locator("#root").evaluate((root) => (root.scrollTop = 220));
  await page.evaluate(() => window.measure());
  await page.waitForFunction(() => window.reports.at(-1)?.available && window.reports.at(-1)?.intervals.length === 0);
  await expect.poll(() => latest(page)).toEqual({
    seq: expect.any(Number),
    available: true,
    selected: true,
    intervals: [],
  });
});

test("a hidden scroll root is unavailable", async ({ page }) => {
  await page.locator("#root").evaluate((root) => (root.style.display = "none"));
  await page.evaluate(() => window.measure());
  await page.waitForFunction(() => window.reports.at(-1)?.available === false);
  await expect.poll(() => latest(page)).toEqual({
    seq: expect.any(Number),
    available: false,
    selected: true,
    intervals: [],
  });
});

test("inactive documents report unselected and unavailable", async ({ page }) => {
  await page.evaluate(() => window.setSelected(false));
  await expect.poll(() => latest(page)).toEqual({
    seq: expect.any(Number),
    available: false,
    selected: false,
    intervals: [],
  });
});
