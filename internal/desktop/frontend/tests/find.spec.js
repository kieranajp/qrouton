import { expect, test } from "@playwright/test";

test("the document shortcut opens a working find bar", async ({ page }) => {
  await page.goto("/tests/find-component.html");
  await page.keyboard.press("Control+f");
  const field = page.getByLabel("Find in document");
  await expect(field).toBeFocused();
  await field.fill("find me");
  await expect(page.getByRole("status")).toHaveText("1 / 3");
  await expect(page.locator("mark[data-document-find]")).toHaveCount(4);

  await field.press("Enter");
  await expect(page.getByRole("status")).toHaveText("2 / 3");
  await field.press("Shift+Enter");
  await expect(page.getByRole("status")).toHaveText("1 / 3");
  await field.press("Escape");
  await expect(field).toHaveCount(0);
  await expect(page.locator("mark[data-document-find]")).toHaveCount(0);
});

test("a document exposes its absolute path and artifact colour", async ({ page }) => {
  await page.goto("/tests/find-component.html");
  await page.getByRole("button", { name: "Copy absolute path" }).click();
  await expect(page.getByRole("button", { name: "Copy absolute path" })).toHaveText("Copied");
  expect(await page.evaluate(() => window.clipboardText)).toBe(
    "/sessions/artifacts/notes/find.md",
  );
  await expect(page.locator('[data-artifact-kind="PLAN"] .face')).toHaveCSS(
    "background-color",
    "rgb(183, 189, 248)",
  );
});

test("find is case-insensitive and crosses inline markup", async ({ page }) => {
  await page.goto("/tests/find.html");
  expect(await page.evaluate(() => window.searchDocument("["))).toBe(0);
  expect(await page.evaluate(() => window.searchDocument("alpha beta"))).toBe(3);
  await expect.poll(() => page.evaluate(() => window.findState())).toMatchObject({
    current: 0,
    marks: [
      { text: "Alpha ", current: true },
      { text: "beta", current: true },
      { text: "ALPHA beta", current: false },
      { text: "alpha beta", current: false },
    ],
  });
});

test("navigation wraps and reveals the current match", async ({ page }) => {
  await page.goto("/tests/find.html");
  await page.evaluate(() => window.searchDocument("alpha beta"));
  expect(await page.evaluate(() => window.moveMatch(-1))).toBe(2);
  await expect.poll(() => page.evaluate(() => window.findState().scrollTop)).toBeGreaterThan(0);
  expect(await page.evaluate(() => window.moveMatch(1))).toBe(0);
});

test("clearing find restores text and inline elements", async ({ page }) => {
  await page.goto("/tests/find.html");
  await page.evaluate(() => {
    window.searchDocument("alpha beta");
    window.clearDocumentSearch();
  });
  await expect(page.locator("mark")).toHaveCount(0);
  await expect(page.locator("#first strong")).toHaveText("beta");
  await expect(page.locator("#first")).toHaveText("Alpha beta appears across markup.");
});
