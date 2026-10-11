import { expect, test } from "@playwright/test";

const SECONDARY = "rgb(165, 173, 203)";

const open = async (page) => {
  await page.goto("/tests/icons.html");
  await page.waitForSelector("svg[data-icon]");
};

const icon = (page, name) => page.locator(`svg[data-icon="${name}"]`);

test("every icon in the set draws as a hidden 16px SVG", async ({ page }) => {
  await open(page);

  const icons = page.locator("svg[data-icon]");
  await expect(icons).toHaveCount(32);
  for (const svg of await icons.all()) {
    await expect(svg).toHaveAttribute("aria-hidden", "true");
    await expect(svg).toHaveAttribute("viewBox", "0 0 16 16");
    const box = await svg.boundingBox();
    expect(box.width).toBe(16);
    expect(box.height).toBe(16);
  }
});

test("a stroke is square-capped, mitred and 1.5 wide", async ({ page }) => {
  await open(page);

  const path = icon(page, "close").locator("path");
  await expect(path).toHaveAttribute("fill", "none");
  await expect(path).toHaveAttribute("stroke", "currentColor");
  await expect(path).toHaveAttribute("stroke-width", "1.5");
  await expect(path).toHaveAttribute("stroke-linecap", "square");
  await expect(path).toHaveAttribute("stroke-linejoin", "miter");
});

test("reference draws an outline and a fill", async ({ page }) => {
  await open(page);

  const paths = icon(page, "reference").locator("path");
  await expect(paths).toHaveCount(2);
  await expect(paths.nth(0)).toHaveAttribute("stroke", "currentColor");
  await expect(paths.nth(1)).toHaveAttribute("fill", "currentColor");
});

test("tree's stroke takes its context's colour and its leaves are teal", async ({ page }) => {
  await open(page);

  const paths = icon(page, "tree").locator("path");
  await expect(paths.nth(0)).toHaveAttribute("stroke", "currentColor");
  await expect(paths.nth(1)).toHaveAttribute("fill", "var(--ctp-teal)");
  expect(await paths.nth(1).evaluate((path) => getComputedStyle(path).fill)).toBe("rgb(139, 213, 202)");
  expect(await icon(page, "tree").evaluate((svg) => getComputedStyle(svg).color)).toBe(SECONDARY);
});

test("the five fixed colours hold and the rest take their context's", async ({ page }) => {
  await open(page);

  const colour = (name) => icon(page, name).evaluate((svg) => getComputedStyle(svg).color);
  expect(await colour("new")).toBe("rgb(238, 212, 159)");
  expect(await colour("agent")).toBe("rgb(138, 173, 244)");
  expect(await colour("shell")).toBe("rgb(138, 173, 244)");
  expect(await colour("pending")).toBe("rgb(110, 115, 141)");
  expect(await colour("excluded")).toBe("rgb(128, 135, 162)");
  expect(await colour("document")).toBe(SECONDARY);
  expect(await colour("command")).toBe(SECONDARY);
});
