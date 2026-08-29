import { expect, test } from "@playwright/test";

const state = (page) => page.evaluate(() => window.chromeState());
const fields = (activity) => ({
  activity,
  sessions: [],
  documents: [],
  repositoryDocuments: [],
  repos: [],
});

test.beforeEach(async ({ page }) => {
  await page.goto("/tests/chrome.html");
  await page.waitForFunction(() => Boolean(window.chromeState));
  expect(await page.evaluate(() => window.chromeCalls)).toEqual([
    "github.com/kieranajp/qrouton/internal/desktop.Chrome.Snapshot",
  ]);
});

test("a live event wins when the snapshot resolves later", async ({ page }) => {
  await page.evaluate((value) => window.emitChrome(value), fields("working"));
  await expect.poll(() => state(page)).toMatchObject({
    fields: { activity: "working" },
    settled: true,
  });
  await page.evaluate((value) => window.resolveChrome(value), fields("idle"));
  await expect.poll(() => state(page)).toMatchObject({ fields: { activity: "working" } });
});

test("a snapshot settles chrome before the first live event", async ({ page }) => {
  await page.evaluate((value) => window.resolveChrome(value), fields("idle"));
  await expect.poll(() => state(page)).toMatchObject({
    fields: { activity: "idle" },
    settled: true,
  });
  await page.evaluate((value) => window.emitChrome(value), fields("waiting"));
  await expect.poll(() => state(page)).toMatchObject({ fields: { activity: "waiting" } });
});
