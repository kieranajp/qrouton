import { expect, test } from "@playwright/test";

const MAC_UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15";
const LINUX_UA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15";

test.use({ userAgent: MAC_UA });

const primary = (page) => page.locator(".primary-frame img");
const decoded = (locator) => locator.evaluate((img) => img.complete && img.naturalWidth > 0);

test("ordered gallery decodes the raster formats and preserves duplicates and captions", async ({ page }) => {
  await page.goto("/tests/assets.html");
  await expect(page.locator(".position")).toHaveText("Image 1 of 9");
  await expect.poll(() => decoded(primary(page))).toBe(true);
  await expect(page.locator(".asset-strip .number")).toHaveText(["1", "2", "3", "4", "5", "6", "7", "8", "9"]);
  await expect(page.locator(".asset-strip .path").nth(0)).toHaveText("thoughts/assets/z/screen.png");
  await expect(page.locator(".asset-strip .path").nth(1)).toHaveText("thoughts/assets/a/screen.png");
  await expect(page.locator(".asset-strip .path").nth(2)).toHaveText("thoughts/assets/z/screen.png");
  await expect.poll(() => page.locator(".asset-strip img").evaluateAll((images) => images.every((img) => img.complete && img.naturalWidth > 0))).toBe(true);
  expect(await page.evaluate(() => window.errors)).toEqual([]);
});

test("clicking the selected image opens a fitted lightbox and Escape closes it", async ({ page }) => {
  await page.goto("/tests/assets.html");
  await expect.poll(() => decoded(primary(page))).toBe(true);
  const preview = await primary(page).boundingBox();

  await primary(page).click();

  const lightbox = page.getByRole("dialog", { name: /Expanded image/ });
  await expect(lightbox).toBeFocused();
  await expect.poll(async () => (await lightbox.locator("img").boundingBox())?.width ?? 0)
    .toBeGreaterThan(preview.width);
  const room = page.viewportSize();
  expect(await lightbox.boundingBox()).toEqual({ x: 0, y: 0, width: room.width, height: room.height });

  await page.keyboard.press("Escape");
  await expect(lightbox).toHaveCount(0);
  await expect(page.locator(".primary-frame")).toBeFocused();
});

test("the lightbox stays open over the image and closes from the scrim or control", async ({ page }) => {
  await page.goto("/tests/assets.html");
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
    await page.goto("/tests/assets.html");
    await expect.poll(() => decoded(primary(page))).toBe(true);
    expect((await primary(page).boundingBox()).width).toBe(48);
    await page.evaluate(() => window.pushAssets({ currentIndex: 2, revision: 2 }));
    await expect.poll(() => decoded(primary(page))).toBe(true);
    const box = await primary(page).boundingBox();
    expect(box.width).toBeLessThanOrEqual(960);
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(width);
    expect(box.width / box.height).toBeCloseTo(1.5, 1);
    const overflow = await page.locator(".asset-strip").evaluate((strip) => strip.scrollWidth > strip.clientWidth);
    if (width === 320) expect(overflow).toBe(true);
  });
}

test("loading and failed image states retain the remaining entries", async ({ page }) => {
  let release;
  const held = new Promise((resolve) => { release = resolve; });
  await page.route("**/small.png?entry=1", async (route) => { await held; await route.abort(); });
  await page.goto("/tests/assets.html", { waitUntil: "domcontentloaded" });
  await expect(page.locator(".primary .loading")).toHaveText("Loading image…");
  release();
  await expect(page.locator(".primary .load-error")).toContainText("Could not load image");
  await expect(page.locator(".primary .load-error")).toContainText("thoughts/assets/z/screen.png");
  await expect(page.locator(".asset-strip li")).toHaveCount(9);
  await expect.poll(() => page.locator(".asset-strip img").count()).toBe(6);
});

test("agent and thumbnail selection share state without agent keyboard focus", async ({ page }) => {
  await page.goto("/tests/assets.html");
  await expect(page.locator(".position")).toHaveText("Image 1 of 9");
  await page.evaluate(() => {
    const input = document.createElement("input");
    input.id = "conversation";
    document.body.prepend(input);
    input.focus();
    window.pushAssets({ currentIndex: 9, revision: 2 });
  });
  await expect(page.locator(".position")).toHaveText("Image 9 of 9");
  await expect(page.locator("#conversation")).toBeFocused();
  const buttons = page.locator(".asset-strip button");
  await expect(buttons.nth(8)).toHaveAttribute("aria-pressed", "true");
  await buttons.nth(2).click();
  await expect(page.locator(".position")).toHaveText("Image 3 of 9");
  await buttons.nth(1).focus();
  await page.keyboard.press("Enter");
  await expect(page.locator(".position")).toHaveText("Image 2 of 9");
  expect(await page.evaluate(() => window.bridgeCalls.filter((call) => call.name.endsWith(".FocusAsset")).map((call) => call.args))).toEqual([["fixture", "assets-1", 3], ["fixture", "assets-1", 2]]);
  await page.evaluate(() => { window.failSelection = true; });
  await buttons.nth(0).click();
  await expect(page.getByRole("alert")).toContainText("Could not select asset");
  await expect(page.locator(".position")).toHaveText("Image 2 of 9");
});

test("newer complete events win over initial responses and reversed revisions", async ({ page }) => {
  await page.goto("/tests/assets.html?delay=1");
  await page.evaluate(() => window.pushAssets({ currentIndex: 9, revision: 4 }));
  await expect(page.locator(".position")).toHaveText("Image 9 of 9");
  await page.evaluate(() => {
    window.pushAssets({ currentIndex: 2, revision: 3 });
    window.releaseInitial();
  });
  await expect(page.locator(".position")).toHaveText("Image 9 of 9");
  await expect(primary(page)).toHaveAttribute("src", /sample.avif/);
});

test("late initial response after replacement cannot restore the old gallery", async ({ page }) => {
  await page.goto("/tests/assets.html?delay=1");
  await page.evaluate(() => window.pushAssets({ currentIndex: 3, revision: 8 }));
  await expect(page.locator(".position")).toHaveText("Image 3 of 9");
  await page.evaluate(() => window.replaceGallery());
  await expect(page.locator(".position")).toHaveText("Image 1 of 9");
  await page.evaluate(() => { window.releaseInitial(); window.pushOldAssets(); });
  await expect(primary(page)).toHaveAttribute("src", /replacement=0/);
  await expect(page.locator(".primary .path")).toHaveText("thoughts/assets/sample.avif");
});

test("corrupt and missing images can be current while other entries remain selectable", async ({ page }) => {
  await page.route("**/sample.webp", (route) => route.fulfill({ contentType: "image/webp", body: "corrupt image bytes" }));
  await page.route("**/small.png?entry=1", (route) => route.fulfill({ status: 404, body: "missing" }));
  await page.goto("/tests/assets.html");
  await expect(page.locator(".primary .load-error")).toContainText("Could not load image");
  await page.locator(".asset-strip button").nth(5).click();
  await expect(page.locator(".position")).toHaveText("Image 6 of 9");
  await expect(page.locator(".primary .load-error")).toContainText("sample.webp");
  await page.locator(".asset-strip button").nth(1).click();
  await expect.poll(() => decoded(primary(page))).toBe(true);
  await page.evaluate(() => { window.failSelection = true; });
  await page.locator(".asset-strip button").nth(0).click();
  await expect(page.getByRole("alert")).toBeVisible();
  await page.evaluate(() => window.replaceGallery());
  await expect(page.locator(".position")).toHaveText("Image 1 of 9");
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(page.locator(".primary .load-error")).toHaveCount(0);
  await expect.poll(() => decoded(primary(page))).toBe(true);
});

test("a pending human request cannot affect a replacement gallery", async ({ page }) => {
  await page.goto("/tests/assets.html");
  await expect(page.locator(".position")).toHaveText("Image 1 of 9");
  await page.evaluate(() => { window.deferSelection = true; });
  await page.locator(".asset-strip button").nth(2).click();
  await expect(page.locator(".position")).toHaveText("Image 1 of 9");
  await page.evaluate(() => window.replaceGallery());
  await expect(primary(page)).toHaveAttribute("src", /replacement=0/);
  await page.evaluate(() => window.releaseFocus());
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(page.locator(".position")).toHaveText("Image 1 of 9");
});

const PLAYABLE = 7;
const MP4 = 8;
const thumbnail = (page, index) => page.locator(".asset-strip li").nth(index - 1).locator("video");
const player = (page) => page.locator(".primary video");

async function selectPlayable(page) {
  await page.goto("/tests/assets.html");
  await page.locator(".asset-strip button").nth(PLAYABLE - 1).click();
  await expect(player(page)).toHaveAttribute("src", /sample-vp9\.mp4/);
  await expect.poll(() => player(page).evaluate((video) => video.readyState)).toBeGreaterThanOrEqual(1);
}

test("the VP9 MP4 thumbnail paints a frame under its play glyph", async ({ page }) => {
  await page.goto("/tests/assets.html");
  const video = thumbnail(page, PLAYABLE);
  await expect(video).toHaveAttribute("src", /sample-vp9\.mp4#t=0\.1$/);
  await expect.poll(() => video.evaluate((v) => v.readyState >= 2 && v.videoWidth > 0)).toBe(true);
  await expect(page.locator(".asset-strip li").nth(PLAYABLE - 1).locator(".play")).toBeVisible();
});

test("selecting the VP9 MP4 shows a paused player without autoplay", async ({ page }) => {
  await selectPlayable(page);
  expect(await player(page).evaluate((video) => ({ paused: video.paused, autoplay: video.autoplay, controls: video.controls })))
    .toEqual({ paused: true, autoplay: false, controls: true });
});

test("selecting another entry removes the playing video", async ({ page }) => {
  await selectPlayable(page);
  const playing = await player(page).elementHandle();
  await playing.evaluate((video) => { video.muted = true; return video.play(); });
  expect(await playing.evaluate((video) => video.paused)).toBe(false);

  await page.locator(".asset-strip button").nth(0).click();

  await expect(page.locator(".position")).toHaveText("Image 1 of 9");
  await expect(player(page)).toHaveCount(0);
  expect(await playing.evaluate((video) => video.isConnected)).toBe(false);
});

test("the position label names a video", async ({ page }) => {
  await page.goto("/tests/assets.html");
  await page.evaluate((index) => window.pushAssets({ currentIndex: index, revision: 2 }), PLAYABLE);
  await expect(page.locator(".position")).toHaveText(`Video ${PLAYABLE} of 9`);
  await expect(page.locator(".asset-strip button").nth(PLAYABLE - 1)).toHaveAccessibleName(/^Video 7: /);
});

test("clicking a video's main view opens no lightbox", async ({ page }) => {
  await selectPlayable(page);
  await expect(page.locator(".primary button")).toHaveCount(0);
  await player(page).click({ position: { x: 20, y: 20 } });
  await expect(page.locator(".lightbox")).toHaveCount(0);
  await expect(page.getByRole("dialog")).toHaveCount(0);
});

test("lightbox stepping skips videos and keeps focus in the lightbox", async ({ page }) => {
  await page.goto("/tests/assets.html");
  await page.evaluate((index) => window.pushAssets({ currentIndex: index, revision: 2 }), PLAYABLE - 1);
  await expect.poll(() => decoded(primary(page))).toBe(true);
  await primary(page).click();
  const lightbox = page.getByRole("dialog", { name: /Expanded image/ });
  await expect(lightbox).toBeFocused();

  await page.keyboard.press("ArrowRight");

  await expect(page.locator(".position")).toHaveText(`Image ${MP4 + 1} of 9`);
  await expect(lightbox).toBeFocused();
  await expect(lightbox.locator("img")).toHaveAttribute("src", /sample\.avif$/);
});

test("the MP4 entry renders as a video", async ({ page }) => {
  await page.goto("/tests/assets.html");
  await expect(thumbnail(page, MP4)).toHaveAttribute("src", /sample\.mp4#t=0\.1$/);
  await page.locator(".asset-strip button").nth(MP4 - 1).click();
  await expect(page.locator(".position")).toHaveText(`Video ${MP4} of 9`);
  await expect(player(page)).toHaveAttribute("src", /sample\.mp4$/);
  await expect(page.locator(".primary img")).toHaveCount(0);
});

const UNPLAYABLE = 10;

async function showUnplayable(page) {
  await page.goto("/tests/assets.html");
  await page.evaluate(() => window.pushUnplayable());
  await expect(page.locator(".position")).toHaveText(`Video ${UNPLAYABLE} of ${UNPLAYABLE}`);
}

async function menuLabels(page, target) {
  await target.click({ button: "right" });
  const labels = page.locator(".anchor .item .label");
  await expect(labels.first()).toBeVisible();
  return labels.allTextContents();
}

test("an undecodable video shows the format failure and a glyph tile", async ({ page }) => {
  await showUnplayable(page);
  await expect(page.locator(".primary .load-error")).toHaveText("This video's format can't be played here");
  await expect(player(page)).toHaveCount(0);
  await expect(page.locator(".primary .loading")).toHaveCount(0);
  const tile = page.locator(".asset-strip li").nth(UNPLAYABLE - 1);
  await expect(tile.locator("video")).toHaveCount(0);
  await expect(tile.locator(".play")).toBeVisible();
  await expect(tile.locator(".load-error")).toHaveCount(0);
});

test("the menu on an undecodable video still offers Copy Path and Reveal", async ({ page }) => {
  await showUnplayable(page);
  await expect(page.locator(".primary .load-error")).toBeVisible();
  expect(await menuLabels(page, page.locator(".primary .load-error"))).toEqual(["Copy", "Copy Path", "Reveal"]);
  await page.locator(".anchor").getByRole("button", { name: "Copy Path", exact: true }).click();
  await expect(page.locator(".notice")).toHaveText("Path copied");
  expect(await page.evaluate(() => window.clipboardText)).toBe("/sessions/fixture/thoughts/assets/not-a-video.mp4");
});

test("Copy on a video says the video was copied", async ({ page }) => {
  await selectPlayable(page);
  await menuLabels(page, page.locator(".asset-strip button").nth(PLAYABLE - 1));
  await page.locator(".anchor").getByRole("button", { name: "Copy", exact: true }).click();
  await expect(page.locator(".notice")).toHaveText("Video copied");
  expect(await page.evaluate(() => window.bridgeCalls.filter((call) => call.name.endsWith(".CopyAsset")).map((call) => call.args)))
    .toEqual([["fixture", "/sessions/fixture/thoughts/assets/sample-vp9.mp4"]]);
});

test("Copy on an image still says the image was copied", async ({ page }) => {
  await page.goto("/tests/assets.html");
  await menuLabels(page, page.locator(".asset-strip button").nth(0));
  await page.locator(".anchor").getByRole("button", { name: "Copy", exact: true }).click();
  await expect(page.locator(".notice")).toHaveText("Image copied");
});

test("the main view keeps loading while only the thumbnail has loaded", async ({ page }) => {
  await page.goto("/tests/assets.html");
  await expect.poll(() => thumbnail(page, PLAYABLE).evaluate((video) => video.readyState)).toBeGreaterThanOrEqual(1);
  await page.route("**/sample-vp9.mp4", () => {});
  await page.locator(".asset-strip button").nth(PLAYABLE - 1).click();
  await expect(player(page)).toHaveAttribute("src", /sample-vp9\.mp4$/);
  await expect(page.locator(".primary .loading")).toHaveText("Loading video…");
  expect(await player(page).evaluate((video) => video.readyState)).toBe(0);
});

test.describe("on Linux", () => {
  test.use({ userAgent: LINUX_UA });

  test("the menu leaves out Copy", async ({ page }) => {
    await page.goto("/tests/assets.html");
    expect(await menuLabels(page, page.locator(".asset-strip button").nth(PLAYABLE - 1))).toEqual(["Copy Path", "Reveal"]);
  });

  test("the format failure names the GStreamer packages", async ({ page }) => {
    await showUnplayable(page);
    const failure = page.locator(".primary .load-error");
    await expect(failure).toContainText("This video's format can't be played here");
    await expect(failure).toContainText("gstreamer1.0-plugins-good");
    await expect(failure).toContainText("gstreamer1.0-libav");
    expect(await menuLabels(page, failure)).toEqual(["Copy Path", "Reveal"]);
  });
});
