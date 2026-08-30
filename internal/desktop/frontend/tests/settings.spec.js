import { expect, test } from "@playwright/test";

const status = (page) => page.locator(".dialog .status");
const root = (page) => page.locator(".dialog input").nth(1);

test("a config that could not be read says so instead of an empty panel", async ({ page }) => {
  await page.goto("/tests/settings.html?fail=Load");
  await expect(page.locator(".dialog")).toBeVisible();

  await expect(status(page)).toContainText("Settings could not be read");
  await expect(status(page)).toContainText("permission denied");
  await expect(root(page)).toHaveValue("");
});

test("a config that answered fills the panel and says nothing", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await expect(root(page)).toHaveValue("/sessions");
  await expect(status(page)).toHaveText("");
});
