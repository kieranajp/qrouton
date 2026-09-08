import { expect, test } from "@playwright/test";

const primary = (page) => page.locator(".primary-frame img");
const decoded = (locator) => locator.evaluate((img) => img.complete && img.naturalWidth > 0);

test("ordered gallery decodes the raster formats and preserves duplicates and captions", async ({ page }) => {
  await page.goto("/tests/images.html");
  await expect(page.locator(".position")).toHaveText("Image 1 of 7");
  await expect.poll(() => decoded(primary(page))).toBe(true);
  await expect(page.locator(".image-strip .number")).toHaveText(["1", "2", "3", "4", "5", "6", "7"]);
  await expect(page.locator(".image-strip .path").nth(0)).toHaveText("thoughts/assets/z/screen.png");
  await expect(page.locator(".image-strip .path").nth(1)).toHaveText("thoughts/assets/a/screen.png");
  await expect(page.locator(".image-strip .path").nth(2)).toHaveText("thoughts/assets/z/screen.png");
  await expect.poll(() => page.locator(".image-strip img").evaluateAll((images) => images.every((img) => img.complete && img.naturalWidth > 0))).toBe(true);
  expect(await page.evaluate(() => window.errors)).toEqual([]);
});

for (const width of [320, 1100]) {
  test(`primary image fits without upscaling at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 720 });
    await page.goto("/tests/images.html");
    await expect.poll(() => decoded(primary(page))).toBe(true);
    expect((await primary(page).boundingBox()).width).toBe(48);
    await page.evaluate(() => window.pushImages({ currentIndex: 2, revision: 2 }));
    await expect.poll(() => decoded(primary(page))).toBe(true);
    const box = await primary(page).boundingBox();
    expect(box.width).toBeLessThanOrEqual(960);
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(width);
    expect(box.width / box.height).toBeCloseTo(1.5, 1);
    const overflow = await page.locator(".image-strip").evaluate((strip) => strip.scrollWidth > strip.clientWidth);
    if (width === 320) expect(overflow).toBe(true);
  });
}

test("loading and failed image states retain the remaining entries", async ({ page }) => {
  let release;
  const held = new Promise((resolve) => { release = resolve; });
  await page.route("**/small.png?entry=1", async (route) => { await held; await route.abort(); });
  await page.goto("/tests/images.html", { waitUntil: "domcontentloaded" });
  await expect(page.locator(".primary .loading")).toHaveText("Loading image…");
  release();
  await expect(page.locator(".primary .load-error")).toContainText("Could not load image");
  await expect(page.locator(".primary .load-error")).toContainText("thoughts/assets/z/screen.png");
  await expect(page.locator(".image-strip li")).toHaveCount(7);
  await expect.poll(() => page.locator(".image-strip img").count()).toBe(6);
});
