import { expect, test } from "@playwright/test";

const open = async (page, query = "") => {
  await page.goto("/tests/spec.html" + query);
  await page.waitForSelector("[data-screen], .markdown", { state: "attached" });
};

const shown = (page) => page.evaluate(() => window.shown());

test("the overview counts the answered questions and lists the decisions", async ({ page }) => {
  await open(page);
  await expect.poll(() => shown(page)).toEqual(["overview"]);
  await expect(page.locator("[data-tally]")).toContainText("1 of 3 answered");
  await expect(page.locator(".rows .row")).toHaveText([/Keep `?Retry`? as a wrapper/, /Cancellation is cooperative/]);
});

test("the questions pip holds every open question as a card", async ({ page }) => {
  await open(page);
  await page.getByRole("button", { name: "Next open question" }).click();
  await expect.poll(() => shown(page)).toEqual(["questions"]);

  const cards = page.locator('[data-screen="questions"] .card');
  await expect(cards).toHaveCount(3);
  await expect(cards.nth(0).locator(".option")).toHaveCount(3);
  await expect(cards.nth(0).locator(".option.chosen")).toContainText("Overall");
  await expect(cards.nth(0).locator(".badge")).toHaveText("Recommended");
  await expect(cards.nth(0).locator(".reason")).toContainText("caps the caller's wait");
  await expect.poll(() => page.evaluate(() => window.focusedCard())).toBe("Q2");
});

test("every decision gets a pip of its own, after the questions", async ({ page }) => {
  await open(page);
  expect(await page.evaluate(() => window.pips())).toEqual([
    "Overview",
    "Open questions",
    "Keep Retry as a wrapper",
    "Cancellation is cooperative",
    "End state",
    "Risks",
  ]);
});

test("arrow keys move between pips, and up and down between cards", async ({ page }) => {
  await open(page);
  await page.keyboard.press("ArrowRight");
  await expect.poll(() => shown(page)).toEqual(["questions"]);
  await expect.poll(() => page.evaluate(() => window.focusedCard())).toBe("Q1");

  await page.keyboard.press("ArrowDown");
  await expect.poll(() => page.evaluate(() => window.focusedCard())).toBe("Q2");
  await page.keyboard.press("j");
  await expect.poll(() => page.evaluate(() => window.focusedCard())).toBe("Q3");
  await page.keyboard.press("k");
  await expect.poll(() => page.evaluate(() => window.focusedCard())).toBe("Q2");

  await page.keyboard.press("ArrowRight");
  await expect.poll(() => shown(page)).toEqual(["Keep Retry as a wrapper"]);
});

test("a spec with nothing open says so and has no questions pip", async ({ page }) => {
  await open(page, "?zero=1");
  await expect(page.locator("[data-tally]")).toHaveText("No open questions");
  expect(await page.evaluate(() => window.pips())).toEqual([
    "Overview",
    "The service writes entries",
    "Append before put",
  ]);
  await expect(page.locator('[data-screen="overview"]')).not.toContainText("None.");
});

test("a spec in no known shape renders as plain markdown", async ({ page }) => {
  await open(page, "?freeform=1");
  await expect(page.locator(".pip")).toHaveCount(0);
  await expect(page.locator(".markdown").first()).toContainText("Do the thing the old way.");
});

test("the document toggle shows the whole text", async ({ page }) => {
  await open(page);
  await page.getByRole("button", { name: "Document" }).click();
  await expect(page.locator(".reading")).toContainText("Per attempt or overall?");
  await expect(page.locator(".reading")).toContainText("Answer: B");
});

test("cards fit the pane without scrolling sideways", async ({ page }) => {
  await page.setViewportSize({ width: 380, height: 800 });
  await open(page);
  await page.keyboard.press("ArrowRight");
  const deck = page.locator(".deck");
  const overflow = await deck.evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(overflow).toBeLessThanOrEqual(0);
});
