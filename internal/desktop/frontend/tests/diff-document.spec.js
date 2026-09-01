import { expect, test } from "@playwright/test";

const file = (name, body = ["-old", "+new"]) => [
  `diff --git a/${name}.txt b/${name}.txt`,
  `--- a/${name}.txt`,
  `+++ b/${name}.txt`,
  "@@ -1 +1 @@",
  ...body,
].join("\n");

async function setDiff(page, text) {
  await page.evaluate((value) => window.setDiff(value), text);
  await expect(page.locator(".diff-document")).toBeVisible();
}

test("Ctrl+F opens FindBar on a diff document through the content bridge", async ({ page }) => {
  await page.goto("/tests/diff-document.html");
  await expect(page.locator(".diff-overview")).toBeVisible();
  await page.keyboard.press("Control+f");
  await expect(page.getByLabel("Find in document")).toBeFocused();
  expect(await page.evaluate(() => window.bridgeCalls)).toEqual([
    { name: "github.com/kieranajp/qrouton/internal/desktop.Chrome.Snapshot", args: [] },
    { name: "github.com/kieranajp/qrouton/internal/desktop.Windows.Content", args: ["document-1"] },
  ]);
});

test("Find keeps its field focused while revealing only the current closed file", async ({ page }) => {
  await page.goto("/tests/diff-document.html");
  await setDiff(page, [
    file("manual", ["-old manual", "+manual"]),
    file("two", ["-old two", "+needle"]),
    file("three", ["-old three", "+needle"]),
  ].join("\n"));
  const summaries = page.locator(".diff-item summary");
  await summaries.first().click();
  await page.keyboard.press("Control+f");
  const field = page.getByLabel("Find in document");
  await field.fill("needle");
  await expect(page.getByRole("status")).toHaveText("1 / 2");
  await expect(field).toBeFocused();
  await expect(page.locator(".diff-file-body")).toHaveCount(2);
  const second = await page.locator(".diff-file-body").nth(1).evaluate((body) => body.closest("details").dataset.file);
  await field.press("Enter");
  await expect(page.getByRole("status")).toHaveText("2 / 2");
  await expect(field).toBeFocused();
  await expect(page.locator(".diff-file-body")).toHaveCount(2);
  const third = await page.locator(".diff-file-body").nth(1).evaluate((body) => body.closest("details").dataset.file);
  expect(third).not.toBe(second);
  await field.press("Shift+Enter");
  await expect(page.getByRole("status")).toHaveText("1 / 2");
  await expect(field).toBeFocused();
});

test("disclosure, labels, modes and line-scoped Files search are accessible", async ({ page }) => {
  await page.goto("/tests/diff-document.html");
  const raw = [
    "outside warning",
    file("inside", ["-old inside", "+needle inside"]),
    "inside warning",
    file("second", ["-old second", "+needle second"]),
  ].join("\n");
  await setDiff(page, raw);
  await expect(page.getByRole("button", { name: "Raw patch" })).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("button", { name: "Files" }).click();
  await expect(page.getByRole("button", { name: "Files" })).toHaveAttribute("aria-pressed", "true");
  const summary = page.locator(".diff-item summary").first();
  expect(await summary.ariaSnapshot()).toContain("inside.txt");
  expect(await summary.ariaSnapshot()).toContain("modified");
  expect(await summary.ariaSnapshot()).toContain("+—");
  await summary.focus();
  await page.keyboard.press("Enter");
  await expect(page.locator(".diff-item").first()).toHaveAttribute("open", "");
  await page.keyboard.press("Space");
  await expect(page.locator(".diff-item").first()).not.toHaveAttribute("open", "");

  await page.keyboard.press("Control+f");
  const field = page.getByLabel("Find in document");
  await field.fill("inside warning");
  await expect(page.getByRole("status")).toHaveText("1 / 1");
  await expect(field).toBeFocused();
  await field.fill("outside warning");
  await expect(page.getByRole("status")).toHaveText("No results");
  await field.fill("-old second\n+needle second");
  await expect(page.getByRole("status")).toHaveText("No results");
  await page.getByRole("button", { name: "Raw patch" }).click();
  await field.focus();
  await field.fill("outside warning");
  await expect(page.getByRole("status")).toHaveText("1 / 1");
});

test("large diff Find remains focused and collapse cancels batched expansion", async ({ page }) => {
  await page.goto("/tests/diff-document.html");
  await setDiff(page, await page.evaluate(() => window.largeDiff()));
  await expect(page.locator(".diff-item")).toHaveCount(220);
  await expect(page.locator(".diff-line")).toHaveCount(0);
  await page.keyboard.press("Control+f");
  const field = page.getByLabel("Find in document");
  await field.fill("+needle");
  await expect(page.getByRole("status")).toHaveText("1 / 220");
  await expect(field).toBeFocused();
  await expect(page.locator(".diff-file-body")).toHaveCount(1);
  const first = await page.locator(".diff-file-body").evaluate((body) => body.closest("details").dataset.file);
  await field.press("Enter");
  await expect(page.getByRole("status")).toHaveText("2 / 220");
  await expect(field).toBeFocused();
  await expect(page.locator(".diff-file-body")).toHaveCount(1);
  const second = await page.locator(".diff-file-body").evaluate((body) => body.closest("details").dataset.file);
  expect(second).not.toBe(first);
  await field.press("Shift+Enter");
  await expect(page.getByRole("status")).toHaveText("1 / 220");
  await expect(page.locator(".diff-file-body")).toHaveCount(1);
  expect(await page.locator(".diff-file-body").evaluate((body) => body.closest("details").dataset.file)).toBe(first);
  await page.getByRole("button", { name: "Expand all" }).click();
  await page.getByRole("button", { name: "Collapse all" }).click();
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  await expect(page.locator(".diff-item[open]")).toHaveCount(0);
  await expect(page.locator(".diff-line")).toHaveCount(0);
});
