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

test("clicking the selected image opens a fitted lightbox and Escape closes it", async ({ page }) => {
  await page.goto("/tests/images.html");
  await expect.poll(() => decoded(primary(page))).toBe(true);
  const preview = await primary(page).boundingBox();

  await primary(page).click();

  const lightbox = page.getByRole("dialog", { name: /Expanded image/ });
  await expect(lightbox).toBeFocused();
  const enlarged = await lightbox.locator("img").boundingBox();
  expect(enlarged.width).toBeGreaterThan(preview.width);
  const room = page.viewportSize();
  expect(await lightbox.boundingBox()).toEqual({ x: 0, y: 0, width: room.width, height: room.height });

  await page.keyboard.press("Escape");
  await expect(lightbox).toHaveCount(0);
  await expect(page.locator(".primary-frame")).toBeFocused();
});

test("the lightbox stays open over the image and closes from the scrim or control", async ({ page }) => {
  await page.goto("/tests/images.html");
  await expect.poll(() => decoded(primary(page))).toBe(true);
  const trigger = page.locator(".primary-frame");

  await trigger.click();
  await page.locator(".lightbox .card img").click({ position: { x: 10, y: 10 } });
  await expect(page.locator(".lightbox")).toHaveCount(1);
  await page.mouse.click(6, 6);
  await expect(page.locator(".lightbox")).toHaveCount(0);
  await expect(trigger).toBeFocused();

  await trigger.click();
  await page.getByRole("button", { name: "Close" }).click();
  await expect(page.locator(".lightbox")).toHaveCount(0);
  await expect(trigger).toBeFocused();
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

test("agent and thumbnail selection share state without agent keyboard focus", async ({ page }) => {
  await page.goto("/tests/images.html");
  await expect(page.locator(".position")).toHaveText("Image 1 of 7");
  await page.evaluate(() => {
    const input = document.createElement("input");
    input.id = "conversation";
    document.body.prepend(input);
    input.focus();
    window.pushImages({ currentIndex: 7, revision: 2 });
  });
  await expect(page.locator(".position")).toHaveText("Image 7 of 7");
  await expect(page.locator("#conversation")).toBeFocused();
  const buttons = page.locator(".image-strip button");
  await expect(buttons.nth(6)).toHaveAttribute("aria-pressed", "true");
  await buttons.nth(2).click();
  await expect(page.locator(".position")).toHaveText("Image 3 of 7");
  await buttons.nth(1).focus();
  await page.keyboard.press("Enter");
  await expect(page.locator(".position")).toHaveText("Image 2 of 7");
  expect(await page.evaluate(() => window.bridgeCalls.filter((call) => call.name.endsWith(".FocusImage")).map((call) => call.args))).toEqual([["fixture", "images-1", 3], ["fixture", "images-1", 2]]);
  await page.evaluate(() => { window.failSelection = true; });
  await buttons.nth(0).click();
  await expect(page.getByRole("alert")).toContainText("Could not select image");
  await expect(page.locator(".position")).toHaveText("Image 2 of 7");
});

test("newer complete events win over initial responses and reversed revisions", async ({ page }) => {
  await page.goto("/tests/images.html?delay=1");
  await page.evaluate(() => window.pushImages({ currentIndex: 7, revision: 4 }));
  await expect(page.locator(".position")).toHaveText("Image 7 of 7");
  await page.evaluate(() => {
    window.pushImages({ currentIndex: 2, revision: 3 });
    window.releaseInitial();
  });
  await expect(page.locator(".position")).toHaveText("Image 7 of 7");
  await expect(primary(page)).toHaveAttribute("src", /sample.avif/);
});

test("late initial response after replacement cannot restore the old gallery", async ({ page }) => {
  await page.goto("/tests/images.html?delay=1");
  await page.evaluate(() => window.pushImages({ currentIndex: 3, revision: 8 }));
  await expect(page.locator(".position")).toHaveText("Image 3 of 7");
  await page.evaluate(() => window.replaceGallery());
  await expect(page.locator(".position")).toHaveText("Image 1 of 7");
  await page.evaluate(() => { window.releaseInitial(); window.pushOldImages(); });
  await expect(primary(page)).toHaveAttribute("src", /replacement=0/);
  await expect(page.locator(".primary .path")).toHaveText("thoughts/assets/sample.avif");
});

test("corrupt and missing images can be current while other entries remain selectable", async ({ page }) => {
  await page.route("**/sample.webp", (route) => route.fulfill({ contentType: "image/webp", body: "corrupt image bytes" }));
  await page.route("**/small.png?entry=1", (route) => route.fulfill({ status: 404, body: "missing" }));
  await page.goto("/tests/images.html");
  await expect(page.locator(".primary .load-error")).toContainText("Could not load image");
  await page.locator(".image-strip button").nth(5).click();
  await expect(page.locator(".position")).toHaveText("Image 6 of 7");
  await expect(page.locator(".primary .load-error")).toContainText("sample.webp");
  await page.locator(".image-strip button").nth(1).click();
  await expect.poll(() => decoded(primary(page))).toBe(true);
  await page.evaluate(() => { window.failSelection = true; });
  await page.locator(".image-strip button").nth(0).click();
  await expect(page.getByRole("alert")).toBeVisible();
  await page.evaluate(() => window.replaceGallery());
  await expect(page.locator(".position")).toHaveText("Image 1 of 7");
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(page.locator(".primary .load-error")).toHaveCount(0);
  await expect.poll(() => decoded(primary(page))).toBe(true);
});

test("a pending human request cannot affect a replacement gallery", async ({ page }) => {
  await page.goto("/tests/images.html");
  await expect(page.locator(".position")).toHaveText("Image 1 of 7");
  await page.evaluate(() => { window.deferSelection = true; });
  await page.locator(".image-strip button").nth(2).click();
  await expect(page.locator(".position")).toHaveText("Image 1 of 7");
  await page.evaluate(() => window.replaceGallery());
  await expect(primary(page)).toHaveAttribute("src", /replacement=0/);
  await page.evaluate(() => window.releaseFocus());
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(page.locator(".position")).toHaveText("Image 1 of 7");
});
