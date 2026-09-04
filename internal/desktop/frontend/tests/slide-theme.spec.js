import { expect, test } from "@playwright/test";

test("a slide resolves the app's own custom properties", async ({ page }) => {
  await page.goto("/tests/slide-theme.html");
  const style = await page.evaluate(() => window.slideStyle("div.marpit > section"));

  expect(style.background).toBe("rgb(1, 2, 3)");
  expect(style.color).toBe("rgb(4, 5, 6)");
});
