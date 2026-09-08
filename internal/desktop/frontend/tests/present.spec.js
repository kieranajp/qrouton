import { expect, test } from "@playwright/test";

const open = async (page) => {
  await page.goto("/tests/slides.html");
  await page.locator(".card").first().waitFor();
};

const start = async (page) => {
  await page.evaluate(() => window.present());
  await page.locator(".present").waitFor();
};

test("Present opens the deck as a card over a scrim, on the counter's slide", async ({ page }) => {
  await open(page);
  await start(page);

  await expect(page.locator(".present-counter")).toHaveText("1 / 7");
  expect(await page.evaluate(() => window.presentHeading())).toBe("The fixture deck");
  const layer = await page.locator(".present").boundingBox();
  const room = page.viewportSize();
  expect(layer.width).toBe(room.width);
  expect(layer.height).toBe(room.height);

  // The scrim reaches the window's edges; the slide does not.
  const box = await page.evaluate(() => window.slideBox());
  for (const side of Object.values(box.inset)) expect(side).toBeGreaterThan(8);
});

test("present mode opens on the slide the reader is standing on", async ({ page }) => {
  await open(page);
  await page.evaluate(() => {
    const cards = document.querySelectorAll(".card");
    window.scroller().scrollTop = cards[3].offsetTop;
  });
  await expect(page.locator(".counter")).toHaveText("4 / 7");
  await start(page);

  await expect(page.locator(".present-counter")).toHaveText("4 / 7");
  expect(await page.evaluate(() => window.presentHeading())).toBe("Fourth");
});

test("the arrows step the deck and stop at both ends", async ({ page }) => {
  await open(page);
  await start(page);

  await page.keyboard.press("ArrowRight");
  await expect(page.locator(".present-counter")).toHaveText("2 / 7");
  expect(await page.evaluate(() => window.presentHeading())).toBe("Second");

  await page.keyboard.press("ArrowLeft");
  await page.keyboard.press("ArrowLeft");
  await expect(page.locator(".present-counter")).toHaveText("1 / 7");

  await page.keyboard.press("End");
  await expect(page.locator(".present-counter")).toHaveText("7 / 7");
  await page.keyboard.press("ArrowRight");
  await expect(page.locator(".present-counter")).toHaveText("7 / 7");

  await page.keyboard.press("Home");
  await expect(page.locator(".present-counter")).toHaveText("1 / 7");
});

test("the workbench is told which note to show, once per step", async ({ page }) => {
  await open(page);
  await start(page);
  await page.waitForFunction(() => window.shows.length > 0);
  const [opening] = await page.evaluate(() => window.shows);

  expect(opening.index).toBe(0);
  expect(opening.total).toBe(7);
  expect(opening.title).toBe("The fixture deck");
  expect(opening.html).toContain("The note under the opener");

  await page.evaluate(() => (window.shows.length = 0));
  await page.keyboard.press("ArrowRight");
  await expect(page.locator(".present-counter")).toHaveText("2 / 7");
  const stepped = await page.evaluate(() => window.shows);

  expect(stepped).toHaveLength(1);
  expect(stepped[0]).toMatchObject({ index: 1, total: 7, title: "Second", html: "" });
});

test("a presenter remote's page keys step the deck too", async ({ page }) => {
  await open(page);
  await start(page);

  await page.keyboard.press("PageDown");
  await expect(page.locator(".present-counter")).toHaveText("2 / 7");
  await page.keyboard.press("PageUp");
  await expect(page.locator(".present-counter")).toHaveText("1 / 7");
});

test("Escape leaves present mode and hands the keyboard back", async ({ page }) => {
  await open(page);
  await start(page);
  await page.keyboard.press("Escape");
  await expect(page.locator(".present")).toHaveCount(0);

  expect(await page.evaluate(() => window.focusedLabel())).toBe("Present");
  await expect(page.locator(".counter")).toHaveText("1 / 7");
});

test("a press on the scrim leaves the presentation, as Escape does", async ({ page }) => {
  await open(page);
  await start(page);
  await page.mouse.click(6, 6);

  await expect(page.locator(".present")).toHaveCount(0);
  await expect(page.locator(".counter")).toHaveText("1 / 7");
});

test("the room around a letterboxed slide is scrim too", async ({ page }) => {
  await open(page);
  await start(page);
  await page.waitForFunction(() => window.slideBox().scale !== 1);
  const box = await page.evaluate(() => window.slideBox());
  const edge = (box.room.width - box.stage.width) / 2;
  expect(box.inset.left).toBeGreaterThan(edge + 4);

  await page.mouse.click((edge + box.inset.left) / 2, box.room.height / 2);
  await expect(page.locator(".present")).toHaveCount(0);
});

test("a press on the slide keeps the presentation up", async ({ page }) => {
  await open(page);
  await start(page);
  await page.locator(".present-slide").click({ position: { x: 40, y: 40 } });

  await expect(page.locator(".present")).toHaveCount(1);
});

test("the arrow controls step the deck and disable at both ends", async ({ page }) => {
  await open(page);
  await start(page);
  const back = page.locator('.present-hud [aria-label="Previous slide"]');
  const on = page.locator('.present-hud [aria-label="Next slide"]');

  await expect(back).toBeDisabled();
  await expect(on).toBeEnabled();

  await on.click();
  await expect(page.locator(".present-counter")).toHaveText("2 / 7");
  expect(await page.evaluate(() => window.presentHeading())).toBe("Second");
  await expect(back).toBeEnabled();

  await back.click();
  await expect(page.locator(".present-counter")).toHaveText("1 / 7");
  await expect(back).toBeDisabled();

  await page.keyboard.press("End");
  await expect(page.locator(".present-counter")).toHaveText("7 / 7");
  await expect(on).toBeDisabled();
  await expect(back).toBeEnabled();
});

test("a step from the arrow controls pushes the note and keeps the keyboard", async ({ page }) => {
  await open(page);
  await start(page);
  await page.waitForFunction(() => window.shows.length > 0);
  await page.evaluate(() => (window.shows.length = 0));

  await page.locator('.present-hud [aria-label="Next slide"]').click();
  await expect(page.locator(".present-counter")).toHaveText("2 / 7");
  expect(await page.evaluate(() => window.shows)).toEqual([
    { index: 1, total: 7, title: "Second", html: "" },
  ]);

  await page.keyboard.press("ArrowRight");
  await expect(page.locator(".present-counter")).toHaveText("3 / 7");
});

test("leaving the deck tab leaves the presentation", async ({ page }) => {
  await open(page);
  await start(page);
  await page.evaluate(() => window.deactivate());

  await expect(page.locator(".present")).toHaveCount(0);
});

test("the Notes control opens the second window and follows it closing", async ({ page }) => {
  await open(page);
  await start(page);
  await page.evaluate(() => window.notes());
  await page.waitForFunction(() => window.opens.length === 1);
  await expect(page.locator(".present-hud [aria-pressed]")).toHaveAttribute("aria-pressed", "true");

  await page.evaluate(() => window.closeNotesWindow());
  await expect(page.locator(".present-hud [aria-pressed]")).toHaveAttribute("aria-pressed", "false");
  await expect(page.locator(".present")).toHaveCount(1);
  expect(await page.evaluate(() => window.closes.length)).toBe(0);
});

test("leaving the presentation takes the notes window with it", async ({ page }) => {
  await open(page);
  await start(page);
  await page.evaluate(() => window.notes());
  await page.waitForFunction(() => window.opens.length === 1);

  await page.keyboard.press("Escape");
  await expect(page.locator(".present")).toHaveCount(0);
  await page.waitForFunction(() => window.closes.length === 1);
});

test("deactivating the deck closes the notes window too", async ({ page }) => {
  await open(page);
  await start(page);
  await page.evaluate(() => window.notes());
  await page.waitForFunction(() => window.opens.length === 1);

  await page.evaluate(() => window.deactivate());
  await expect(page.locator(".present")).toHaveCount(0);
  await page.waitForFunction(() => window.closes.length === 1);
});

test("closing the deck tab takes the presentation with it", async ({ page }) => {
  await open(page);
  await start(page);
  await page.evaluate(() => window.notes());
  await page.waitForFunction(() => window.opens.length === 1);

  await page.evaluate(() => window.closeTab());
  await expect(page.locator(".present")).toHaveCount(0);
  await page.waitForFunction(() => window.closes.length === 1);
});

test("no app shortcut fires from under the presentation", async ({ page }) => {
  await open(page);
  await page.keyboard.press("Control+Comma");
  expect(await page.evaluate(() => window.appKeys.length)).toBeGreaterThan(0);

  await start(page);
  await page.evaluate(() => (window.appKeys.length = 0));
  await page.keyboard.press("Control+Comma");
  await page.keyboard.press("Meta+1");
  await page.keyboard.press("ArrowRight");

  expect(await page.evaluate(() => window.appKeys)).toEqual([]);
  await expect(page.locator(".present-counter")).toHaveText("2 / 7");
});

test("a deck edited down to fewer slides carries the presentation back", async ({ page }) => {
  await open(page);
  await start(page);
  await page.keyboard.press("End");
  await expect(page.locator(".present-counter")).toHaveText("7 / 7");

  await page.evaluate(() => window.shorten());
  await expect(page.locator(".present-counter")).toHaveText("2 / 2");
  expect(await page.evaluate(() => window.presentHeading())).toBe("Last");
});

test("find declines to open behind the presentation", async ({ page }) => {
  await open(page);
  await start(page);
  await page.keyboard.press("Control+f");

  await expect(page.locator(".find")).toHaveCount(0);
});

for (const room of [
  { width: 1400, height: 600 },
  { width: 800, height: 1000 },
]) {
  test(`the slide keeps Marp's pixel box inside a ${room.width}x${room.height} window`, async ({
    page,
  }) => {
    await page.setViewportSize(room);
    await open(page);
    await start(page);
    await page.waitForFunction(() => window.slideBox().scale !== 1);
    const box = await page.evaluate(() => window.slideBox());

    expect(box.declared).toBe(1280);
    expect(box.width / box.height).toBeCloseTo(16 / 9, 2);
    expect(box.stage.width).toBeLessThan(room.width);
    expect(box.stage.height).toBeLessThan(room.height);
    expect(box.width).toBeLessThanOrEqual(box.stage.width + 1);
    expect(box.height).toBeLessThanOrEqual(box.stage.height + 1);
    // Fitted to the inset box, not merely contained: one dimension is against
    // the wall.
    const filled = Math.max(box.width / box.stage.width, box.height / box.stage.height);
    expect(filled).toBeGreaterThan(0.99);
  });
}
