import { expect, test } from "@playwright/test";

const open = async (page, query = "") => {
  await page.goto(`/tests/slides.html${query}`);
  await page.locator(".card").first().waitFor();
};

test("a deck draws one card per slide, each holding 16:9 in flow", async ({ page }) => {
  await open(page);
  const cards = await page.evaluate(() => window.cards());

  expect(cards).toHaveLength(7);
  for (const card of cards) {
    expect(card.frame.width / card.frame.height).toBeCloseTo(16 / 9, 2);
  }
});

test("cards carry ascending source spans", async ({ page }) => {
  await open(page);
  const cards = await page.evaluate(() => window.cards());
  const lines = cards.map((card) => card.line);

  expect(lines[0]).toBe(4);
  expect(lines).toEqual([...lines].sort((a, b) => a - b));
  for (const card of cards) expect(card.lineEnd).toBeGreaterThanOrEqual(card.line);
});

test("a note renders below its card and never resizes it", async ({ page }) => {
  await open(page);
  const cards = await page.evaluate(() => window.cards());

  expect(cards[0].notes).toBeGreaterThan(0);
  expect(cards[1].notes).toBe(0);
  // The third slide's note is forty sentences long and its card is the same
  // height as its neighbours', which carry none.
  expect(cards[2].notes).toBeGreaterThan(cards[0].notes);
  expect(cards[2].frame.height).toBeCloseTo(cards[1].frame.height, 1);
});

test("a narrower pane rescales the slide rather than reflowing it", async ({ page }) => {
  await open(page);
  const wide = await page.evaluate(() => window.cards());
  await page.evaluate(() => window.narrow());
  await page.waitForFunction(
    (before) => window.cards()[0].frame.width < before,
    wide[0].frame.width,
  );
  const narrow = await page.evaluate(() => window.cards());

  expect(narrow[0].frame.width / narrow[0].frame.height).toBeCloseTo(16 / 9, 2);
  const scale = await page.evaluate(() => {
    const slide = document.querySelector(".card .marpit");
    return { width: slide.getBoundingClientRect().width, declared: slide.offsetWidth };
  });
  expect(scale.declared).toBe(1280);
  expect(scale.width).toBeCloseTo(narrow[0].frame.width, 0);
});

test("a requested line marks and reveals its own card", async ({ page }) => {
  await open(page, "?line=39&to=39");
  const marked = await page.evaluate(() =>
    [...document.querySelectorAll(".card.marked")].map((card) => Number(card.dataset.line)),
  );

  expect(marked).toEqual([38]);
  await expect(page.locator(".card.marked")).toBeInViewport();
});

test("the counter names the card the reader is standing on", async ({ page }) => {
  await open(page);
  await expect(page.locator(".counter")).toHaveText("1 / 7");

  await page.evaluate(() => {
    const cards = document.querySelectorAll(".card");
    window.scroller().scrollTop = cards[3].offsetTop;
  });
  await expect(page.locator(".counter")).not.toHaveText("1 / 7");
});

test("relative media resolves over the deck's asset route", async ({ page }) => {
  await open(page);
  await page.evaluate(() =>
    window.pushDeck('<img src="./shot.png">\n<video src="./clip.mp4"></video>\n\n![](../shared/plate.png)\n'),
  );
  await page.locator(".card video").waitFor({ state: "attached" });
  const media = await page.evaluate(() =>
    [...document.querySelectorAll(".card section img, .card section video")].map((el) =>
      el.getAttribute("src"),
    ),
  );

  expect(media).toEqual([
    "/deck/tok/shot.png",
    "/deck/tok/clip.mp4",
    "/deck/tok/../shared/plate.png",
  ]);
});
