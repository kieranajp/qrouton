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
  const wideScale = await page.evaluate(() => window.slideScale());
  await page.evaluate(() => window.narrow());
  // The factor, not the frame: the frame narrows in the same layout the width
  // was set in, while the factor waits on the observer that measures it.
  await page.waitForFunction((before) => window.slideScale() < before, wideScale);
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
  await expect(page.locator(".footer .counter")).toHaveText("Slide 7 of 7");
  await expect(page.getByRole("button", { name: "Slide 7", exact: true })).toHaveAttribute("aria-current", "true");
  await expect(page.locator(".footer [data-line]")).toHaveCount(0);
});

test("the counter names the card the reader is standing on", async ({ page }) => {
  await open(page);
  await expect(page.locator(".counter")).toHaveText("Slide 1 of 7");

  await page.evaluate(() => {
    const cards = document.querySelectorAll(".card");
    cards[3].scrollIntoView({ block: "start" });
  });
  await expect(page.locator(".counter")).toHaveText("Slide 4 of 7");
  await expect(page.getByRole("button", { name: "Slide 4", exact: true })).toHaveAttribute("aria-current", "true");
});

test("a source reveal selects its card and then leaves manual scrolling to the reader", async ({ page }) => {
  await page.setViewportSize({ width: 900, height: 700 });
  await open(page, "?line=13&to=13");
  await expect(page.locator(".counter")).toHaveText("Slide 2 of 7");
  await page.evaluate(() => { window.scroller().scrollTop = 0; });
  await expect(page.locator(".counter")).toHaveText("Slide 1 of 7");
});

test("footer pips and arrows select cards and stop at both boundaries", async ({ page }) => {
  await open(page);
  await expect(page.locator(".footer .pip")).toHaveCount(7);
  await expect(page.locator('.pip[aria-current="true"]')).toHaveCount(1);
  await expect(page.locator(".source .counter")).toHaveCount(0);
  await expect(page.locator(".source button", { hasText: "Present" })).toHaveCount(0);
  const previous = page.getByRole("button", { name: "Previous slide" });
  const next = page.getByRole("button", { name: "Next slide" });
  await expect(previous).toBeDisabled();
  await expect(next).toBeEnabled();
  await page.getByRole("button", { name: "Slide 4", exact: true }).click();
  await expect(page.locator(".counter")).toHaveText("Slide 4 of 7");
  await expect(page.locator(".card").nth(3)).toBeInViewport();
  await next.click();
  await expect(page.locator(".counter")).toHaveText("Slide 5 of 7");
  await previous.click();
  await expect(page.locator(".counter")).toHaveText("Slide 4 of 7");
  await page.getByRole("button", { name: "Slide 7", exact: true }).click();
  await expect(page.locator(".counter")).toHaveText("Slide 7 of 7");
  await expect(next).toBeDisabled();
  await expect(previous).toBeEnabled();
});

test("the bottom selects the final card even with the previous card visible", async ({ page }) => {
  await page.setViewportSize({ width: 900, height: 700 });
  await open(page);
  await page.evaluate(() => {
    window.scroller().scrollTop = window.scroller().scrollHeight;
  });
  await expect(page.locator(".card").nth(5)).toBeInViewport();
  await expect(page.locator(".counter")).toHaveText("Slide 7 of 7");
  await expect(page.locator('.pip[aria-current="true"]')).toHaveAttribute("aria-label", "Slide 7");
});

test("shortening a deck clamps all footer controls and Present together", async ({ page }) => {
  await open(page);
  await page.getByRole("button", { name: "Slide 7", exact: true }).click();
  await page.evaluate(() => window.shorten());
  await expect(page.locator(".card")).toHaveCount(2);
  await expect(page.locator(".pip")).toHaveCount(2);
  await expect(page.locator(".counter")).toHaveText("Slide 2 of 2");
  await expect(page.locator('.pip[aria-current="true"]')).toHaveAttribute("aria-label", "Slide 2");
  await expect(page.getByRole("button", { name: "Next slide" })).toBeDisabled();
  await page.getByRole("button", { name: "Present", exact: true }).click();
  await expect(page.locator(".present-counter")).toHaveText("2 / 2");
});

test("a single slide disables both arrows", async ({ page }) => {
  await open(page);
  await page.evaluate(() => window.pushDeck("# Only slide\n"));
  await expect(page.locator(".counter")).toHaveText("Slide 1 of 1");
  await expect(page.locator(".pip")).toHaveCount(1);
  await expect(page.getByRole("button", { name: "Previous slide" })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Next slide" })).toBeDisabled();
});

test("rendered cards without source spans still select by their card index", async ({ page }) => {
  await open(page);
  await page.evaluate(() => window.pushDeck("---\nmarp: true\nheadingDivider: 2\n---\n\n## First\n\n## Second\n\n## Third\n"));
  await expect(page.locator(".card")).toHaveCount(3);
  await expect(page.locator(".card").nth(1)).not.toHaveAttribute("data-line");
  await page.getByRole("button", { name: "Slide 2", exact: true }).click();
  await expect(page.locator(".counter")).toHaveText("Slide 2 of 3");
  await page.evaluate(() => document.querySelectorAll(".card")[2].scrollIntoView({ block: "start" }));
  await expect(page.locator(".counter")).toHaveText("Slide 3 of 3");
});

for (const width of [900, 400]) {
  test(`the footer stays fixed and its pips keep one row at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 700 });
    await open(page);
    const before = await page.evaluate(() => window.footerShape());
    expect(before).toMatchObject({ gap: 0, left: 0, outerScroll: 0, outerOverflows: false, innerOverflows: true, pipRows: 1, controlsFit: true });
    expect(before.width).toBe(before.paneWidth);
    expect(before.scrollerBottom).toBe(before.footerTop);
    await page.getByRole("button", { name: "Slide 7", exact: true }).click();
    await expect(page.locator(".counter")).toHaveText("Slide 7 of 7");
    expect(await page.evaluate(() => window.footerShape())).toEqual(before);
  });
}

test("the footer controls fit a pane narrowed independently of the viewport", async ({ page }) => {
  await page.setViewportSize({ width: 900, height: 700 });
  await open(page);
  await page.evaluate(() => window.narrow());
  const shape = await page.evaluate(() => window.footerShape());
  expect(shape).toMatchObject({ paneWidth: 480, pipRows: 1, controlsFit: true, gap: 0 });
  await page.getByRole("button", { name: "Slide 7", exact: true }).click();
  await expect(page.locator(".counter")).toHaveText("Slide 7 of 7");
});

test("find reveals a slide through the reader-owned scroller", async ({ page }) => {
  await open(page);
  await page.keyboard.press("Control+f");
  await page.getByLabel("Find in document").fill("Body copy on the seventh");
  await expect(page.locator("mark[data-document-find].current")).toBeInViewport();
  await expect(page.locator(".counter")).toHaveText("Slide 7 of 7");
  const shape = await page.evaluate(() => window.footerShape());
  expect(shape.outerScroll).toBe(0);
  expect(shape.gap).toBe(0);
  await page.evaluate(() => { window.scroller().scrollTop = 0; });
  await expect(page.locator(".counter")).toHaveText("Slide 1 of 7");
  await page.getByLabel("Find in document").press("Escape");
  await expect(page.locator(".preview")).toBeFocused();
});

test("find keeps a match at the end of long notes visible", async ({ page }) => {
  await open(page);
  await page.evaluate(() => window.pushDeck(`# First\n\n---\n\n## Notes\n\n<!-- ${"Long notes. ".repeat(800)} unique ending -->\n\n---\n\n# Last\n`));
  await page.keyboard.press("Control+f");
  await page.getByLabel("Find in document").fill("unique ending");
  await expect(page.locator(".counter")).toHaveText("Slide 2 of 3");
  await expect(page.locator("mark[data-document-find].current")).toBeInViewport();
});

test("a default slide reads as a card against the pane's own ground", async ({ page }) => {
  await open(page);
  const info = await page.evaluate(() => {
    // Card 0 opens on the title layout; card 1 is the deck's default layout.
    const deck = document.querySelector(".deck");
    const frame = document.querySelectorAll(".frame")[1];
    const section = frame.querySelector(".marpit section");
    return {
      paneGround: getComputedStyle(deck).backgroundColor,
      slideBackground: getComputedStyle(section).backgroundColor,
      outlineWidth: getComputedStyle(frame).outlineWidth,
      outlineColor: getComputedStyle(frame).outlineColor,
    };
  });

  expect(info.slideBackground).not.toBe(info.paneGround);
  expect(info.outlineWidth).not.toBe("0px");
  expect(info.outlineColor).not.toBe(info.slideBackground);
  expect(info.outlineColor).not.toBe(info.paneGround);
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

test("a d2 fence waits on the slide, then draws fitted inside it", async ({ page }) => {
  await open(page);
  await page.evaluate(() => window.pushDiagramDeck());
  await page.locator(".card pre[data-line]").waitFor();

  const waiting = await page.evaluate(() => window.diagram());
  expect(waiting.line).toBe("7");
  expect(waiting.lineEnd).toBe("9");
  expect(waiting.pending).toBe(true);
  expect(waiting.code).toBe(true);

  const drawn = await page.evaluate(() => (window.drawDiagram(), window.diagram()));
  expect(drawn.drawn).toBe(true);
  expect(drawn.pending).toBe(false);
  expect(drawn.code).toBe(false);
  expect(drawn.width).toBeGreaterThan(0);
  expect(drawn.width).toBeLessThanOrEqual(drawn.slideWidth);
  expect(drawn.height).toBeLessThanOrEqual(drawn.slideHeight);
});

test("a diagram on a slide has no view of its own to pan or zoom", async ({ page }) => {
  await open(page);
  await page.evaluate(() => window.pushDiagramDeck());
  await page.locator(".card pre[data-line]").waitFor();
  const drawn = await page.evaluate(() => (window.drawDiagram(), window.diagram()));

  expect(drawn.staged).toBe(false);
  expect(drawn.zoomable).toBe(false);
  expect(drawn.controls).toBe(0);
  expect(drawn.styleWidth).toBe("");
  expect(drawn.viewBox).toContain(drawn.attrWidth);
});

test("a fence that failed states its reason on the slide", async ({ page }) => {
  await open(page);
  await page.evaluate(() => window.pushDiagramDeck());
  await page.locator(".card pre[data-line]").waitFor();
  const failed = await page.evaluate(
    () => (window.failDiagram("5:1: <b> is not a shape"), window.diagram()),
  );

  expect(failed.failed).toBe(true);
  expect(failed.pending).toBe(false);
  expect(failed.error).toBe("5:1: <b> is not a shape");
});
