import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.goto("/tests/terminal-focus.html");
  await expect(page.locator("#conversation")).toBeFocused();
});

test("agent activation reveals and refits a terminal without taking the keyboard", async ({ page }) => {
  await page.evaluate(() => window.agentSelect());
  await expect(page.locator("#terminal-pane")).toBeVisible();
  await expect.poll(() => page.evaluate(() => window.refits)).toBe(1);
  await expect(page.locator("#conversation")).toBeFocused();
});

test("each user selection focuses the terminal, including an already active one", async ({ page }) => {
  await page.evaluate(() => window.agentSelect());
  await page.locator("#user-select").click();
  await expect(page.locator("#terminal")).toBeFocused();
  await expect.poll(() => page.evaluate(() => window.focuses)).toBe(1);

  await page.locator("#conversation").focus();
  await page.locator("#user-select").click();
  await expect(page.locator("#terminal")).toBeFocused();
  await expect.poll(() => page.evaluate(() => window.focuses)).toBe(2);
});

test("focus requested before mount is delivered once and acknowledged across remount", async ({ page }) => {
  await page.evaluate(() => window.userSelectBeforeMount());
  await expect(page.locator("#terminal")).toBeFocused();
  await expect.poll(() => page.evaluate(() => window.focuses)).toBe(1);

  await page.locator("#conversation").focus();
  await page.evaluate(() => window.remount());
  await expect.poll(() => page.evaluate(() => window.refits)).toBe(2);
  await expect(page.locator("#conversation")).toBeFocused();
  await expect.poll(() => page.evaluate(() => window.focuses)).toBe(1);
});
