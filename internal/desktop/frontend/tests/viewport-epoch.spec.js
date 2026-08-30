import { expect, test } from "@playwright/test";

const epoch = (page) => page.evaluate(() => window.reports.at(-1)?.epoch);
const seen = (page) => page.evaluate(() => window.reports.length);

// The workbench fences a viewport report against the epoch it names, so a pane
// that read its epoch once at attach reports every later position under the
// epoch the document had before the push, and the workbench drops all of them.
test("a report made after a content push carries the epoch that push brought", async ({ page }) => {
  await page.goto("/tests/viewport-epoch.html");
  await expect.poll(() => epoch(page)).toBe(1);

  await page.evaluate(() => window.pushEpoch(7));
  const before = await seen(page);
  await page.evaluate(() => window.scrollTo_(600));

  await expect.poll(() => seen(page)).toBeGreaterThan(before);
  await expect.poll(() => epoch(page)).toBe(7);
});
