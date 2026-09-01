import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.goto("/tests/diff.html");
  await expect(page.locator(".diff-item")).toHaveCount(2);
  await expect(page.locator(".diff-line")).toHaveCount(0);
});

test("opens a compact overview and only mounts the files the reader opens", async ({ page }) => {
  await expect(page.locator(".diff-overview")).toContainText("2 files");
  await expect(page.getByRole("button", { name: "Files" })).toHaveAttribute("aria-pressed", "true");
  await page.locator(".diff-item summary").nth(0).click();
  const firstRows = await page.locator(".diff-line").count();
  expect(firstRows).toBeGreaterThan(0);
  await page.locator(".diff-item summary").nth(1).click();
  expect(await page.locator(".diff-line").count()).toBeGreaterThan(firstRows);
  await expect(page.locator(".diff-item").nth(0)).toHaveAttribute("open", "");
  await expect(page.locator(".diff-item").nth(1)).toHaveAttribute("open", "");
});

test("keeps opened files while switching to the exact raw patch stream", async ({ page }) => {
  await page.locator(".diff-item summary").nth(0).click();
  await page.getByRole("button", { name: "Raw patch" }).click();
  await expect(page.locator(".diff-raw")).toHaveText(await page.evaluate(() => window.defaultDiff));
  expect(await page.evaluate(() => window.selectDiff())).toBe(await page.evaluate(() => window.defaultDiff));
  await expect(page.getByRole("button", { name: "Raw patch" })).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("button", { name: "Files" }).click();
  await expect(page.locator(".diff-item").nth(0)).toHaveAttribute("open", "");
});

test("keeps numbered gutters inside an opened body at narrow widths", async ({ page }) => {
  await page.locator(".diff-item summary").first().click();
  await page.evaluate(() => window.setPaneWidth(180));
  const measurements = await page.locator(".diff-file-body .diff-grid").evaluate((grid) => {
    const row = grid.querySelectorAll(".diff-line")[7];
    const oldGutter = row.querySelector(".diff-old");
    const newGutter = row.querySelector(".diff-new");
    const content = row.querySelector(".diff-content");
    return {
      clientWidth: grid.clientWidth,
      scrollWidth: grid.scrollWidth,
      old: oldGutter.getBoundingClientRect().toJSON(),
      next: newGutter.getBoundingClientRect().toJSON(),
      content: content.getBoundingClientRect().toJSON(),
      oldValue: oldGutter.dataset.line,
    };
  });
  expect(measurements.scrollWidth).toBeGreaterThanOrEqual(measurements.clientWidth);
  expect(measurements.oldValue).toBe("99");
  expect(measurements.old.right).toBeLessThanOrEqual(measurements.next.left + 1);
  expect(measurements.next.right).toBeLessThanOrEqual(measurements.content.left + 1);
});

test("uses native disclosure semantics and bulk actions", async ({ page }) => {
  const summary = page.locator(".diff-item summary").first();
  await summary.focus();
  await page.keyboard.press("Space");
  await expect(page.locator(".diff-item").first()).toHaveAttribute("open", "");
  await page.getByRole("button", { name: "Collapse all" }).click();
  await expect(page.locator(".diff-line")).toHaveCount(0);
  await page.getByRole("button", { name: "Expand all" }).click();
  await expect(page.locator(".diff-line")).not.toHaveCount(0);
});

test("keeps a large patch unmounted until a file is opened", async ({ page }) => {
  await page.evaluate(async () => {
    const files = Array.from({ length: 120 }, (_, index) => [
      `diff --git a/file-${index}.txt b/file-${index}.txt`,
      "index 1111111..2222222 100644",
      `--- a/file-${index}.txt`,
      `+++ b/file-${index}.txt`,
      "@@ -1 +1 @@",
      "-old line",
      "+new line",
    ].join("\n"));
    await window.setDiff(files.join("\n"));
  });
  await expect(page.locator(".diff-item")).toHaveCount(120);
  await expect(page.locator(".diff-line")).toHaveCount(0);
  await page.locator(".diff-item summary").nth(88).click();
  await expect(page.locator(".diff-line")).toHaveCount(8);
});

test("starts partial output in Raw patch while keeping Files available", async ({ page }) => {
  await page.evaluate(async () => window.setDiff(`leading warning\n${window.defaultDiff}`));
  await expect(page.locator(".diff-raw")).toContainText("leading warning");
  await expect(page.getByRole("button", { name: "Raw patch" })).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByRole("button", { name: "Files" })).toBeEnabled();
  await page.getByRole("button", { name: "Files" }).click();
  await expect(page.locator(".diff-notice")).toBeVisible();
});

test("Find reveals only its current closed file and Raw patch can match across lines", async ({ page }) => {
  await page.evaluate(async () => {
    await window.setDiff([
      "diff --git a/one.txt b/one.txt", "--- a/one.txt", "+++ b/one.txt", "@@ -1 +1 @@", "-old one", "+needle one",
      "diff --git a/two.txt b/two.txt", "--- a/two.txt", "+++ b/two.txt", "@@ -1 +1 @@", "-old two", "+needle two",
    ].join("\n"));
  });
  expect(await page.evaluate(() => window.diffFindAdapter.refresh("needle"))).toEqual({ count: 2, current: 0 });
  await expect(page.locator(".diff-file-body")).toHaveCount(1);
  expect(await page.evaluate(() => window.diffFindAdapter.move(1))).toEqual({ count: 2, current: 1 });
  await expect(page.locator(".diff-file-body")).toHaveCount(1);
  await page.getByRole("button", { name: "Raw patch" }).click();
  expect(await page.evaluate(() => window.diffFindAdapter.refresh("old two\n+needle"))).toEqual({ count: 1, current: 0 });
  await expect(page.locator(".diff-raw mark")).toHaveCount(1);
});
