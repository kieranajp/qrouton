import { expect, test } from "@playwright/test";

// The dialog takes the keyboard, so Enter answers whichever button holds it.
test("Enter answers the destructive button the dialog was opened to offer", async ({ page }) => {
  await page.goto("/tests/confirm.html");

  await expect.poll(() => page.evaluate(() => window.focused())).toMatchObject({
    text: "Delete",
    variant: expect.stringContaining("destructive"),
  });

  await page.keyboard.press("Enter");
  expect(await page.evaluate(() => window.answers)).toEqual(["confirm"]);
});

test("Escape leaves the session alone", async ({ page }) => {
  await page.goto("/tests/confirm.html");
  await expect.poll(() => page.evaluate(() => window.focused().text)).toBe("Delete");

  await page.keyboard.press("Escape");
  expect(await page.evaluate(() => window.answers)).toEqual(["cancel"]);
});
