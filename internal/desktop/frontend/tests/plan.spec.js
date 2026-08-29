import { expect, test } from "@playwright/test";

const RUNNING = "rgb(139, 213, 202)";

const open = async (page, query = "") => {
  await page.goto("/tests/plan.html" + query);
  await page.waitForSelector("[data-screen], .markdown", { state: "attached" });
};

const shown = (page) => page.evaluate(() => window.shown());
const marked = (page) => page.evaluate(() => window.markedLines());

test("a span in a later phase opens that phase and marks only its blocks", async ({ page }) => {
  await open(page, "?line=19&to=21");

  await expect.poll(() => shown(page)).toEqual(["2"]);
  await expect.poll(() => marked(page)).toEqual([19, 21]);

  await expect
    .poll(() => page.evaluate(() => window.reports.at(-1)))
    .toMatchObject({ available: true, selected: true });
  const intervals = await page.evaluate(() => window.reports.at(-1).intervals);
  expect(intervals.length).toBeGreaterThan(0);
  for (const interval of intervals) {
    expect(interval.line).toBeGreaterThanOrEqual(17);
    expect(interval.to).toBeLessThanOrEqual(38);
  }

  await page.keyboard.press("ArrowRight");
  await expect.poll(() => shown(page)).toEqual(["3"]);
  expect(await marked(page)).toEqual([]);
});

test("a span in the preamble opens the overview", async ({ page }) => {
  await open(page, "?line=7");
  await expect.poll(() => shown(page)).toEqual(["overview"]);
  await expect.poll(() => marked(page)).toEqual([7]);
});

test("a span crossing a boundary marks only the part inside its opening phase", async ({ page }) => {
  await open(page, "?line=36&to=41");
  await expect.poll(() => shown(page)).toEqual(["2"]);
  await expect.poll(() => marked(page)).toEqual([36, 37]);
});

test("the overview lists every phase with the count its criteria state", async ({ page }) => {
  await open(page);
  await expect.poll(() => shown(page)).toEqual(["overview"]);
  expect(await page.evaluate(() => window.counters())).toEqual([
    "1 Groundwork 2/2",
    "2 The middle 1/2",
    "3 The end 0/1",
  ]);
});

test("observations render in the body and stay out of the meter", async ({ page }) => {
  await open(page, "?line=19");
  const screen = page.locator('[data-screen="2"]');

  await expect(screen.locator(".markdown:not(.lifted)").first()).toContainText("the deck looks right");
  await expect(screen.locator(".markdown:not(.lifted)").first()).toContainText("the pips line up");
  await expect(screen.locator('[data-count="2"]')).toHaveText("1 of 2 met");
  await expect(screen.locator(".state")).toContainText("Working");
  await expect(screen.locator(".state .dot")).toHaveCSS("background-color", RUNNING);
});

test("arrow keys move between screens and only the shown one is drawn", async ({ page }) => {
  await open(page);
  await expect.poll(() => page.evaluate(() => window.drawnScreens())).toEqual(["overview"]);

  await page.keyboard.press("ArrowRight");
  await expect.poll(() => shown(page)).toEqual(["1"]);
  await expect.poll(() => page.evaluate(() => window.drawnScreens())).toEqual(["1"]);

  await page.keyboard.press("ArrowLeft");
  await expect.poll(() => shown(page)).toEqual(["overview"]);
});

test("a fenced block and a table inside a phase body render as themselves", async ({ page }) => {
  await open(page, "?line=19");
  const screen = page.locator('[data-screen="2"]');

  await expect(screen.locator("pre")).toHaveCount(1);
  await expect(screen.locator("pre")).toContainText("func main()");
  await expect(screen.locator("table tbody tr")).toHaveCount(1);
  await expect(screen.locator("table")).toContainText("right");
});

test("a document nothing opens a phase in renders the plain markdown body", async ({ page }) => {
  await open(page, "?plain=true");
  await expect(page.locator("[data-screen]")).toHaveCount(0);
  await expect(page.locator(".markdown")).toContainText("No headings open anything here.");
});

test("a pip selects its phase and takes the underline", async ({ page }) => {
  await open(page);
  const pip = page.locator('.pip[aria-label="Phase 3"]');

  await pip.click();
  await expect.poll(() => shown(page)).toEqual(["3"]);
  await expect(pip).toHaveAttribute("aria-current", "true");
  await expect(pip).toHaveCSS("border-bottom-color", "rgb(138, 173, 244)");
  await expect(page.locator('.pip[aria-label="Phase 1"]')).toHaveCSS(
    "border-bottom-color",
    "rgba(0, 0, 0, 0)",
  );
});

test("the counter names the overview and the arrows stop at both ends", async ({ page }) => {
  await open(page);
  const previous = page.locator('button[aria-label="Previous screen"]');
  const next = page.locator('button[aria-label="Next screen"]');

  await expect(page.locator(".counter")).toHaveText("Overview");
  await expect(previous).toBeDisabled();
  await expect(next).toBeEnabled();

  await next.click();
  await expect(page.locator(".counter")).toHaveText("1 / 3");
  await next.click();
  await next.click();
  await expect(page.locator(".counter")).toHaveText("3 / 3");
  await expect(next).toBeDisabled();
  await expect(previous).toBeEnabled();
});

test("raw shows the source and coming back lands on the same phase", async ({ page }) => {
  await open(page, "?line=19");
  await expect.poll(() => shown(page)).toEqual(["2"]);

  await page.getByRole("button", { name: "Raw", exact: true }).click();
  await expect.poll(() => page.evaluate(() => window.mode())).toBe("raw");
  await expect(page.locator("pre.raw")).toContainText("## Phase 2 — The middle");

  await page.getByRole("button", { name: "Plan", exact: true }).click();
  await expect.poll(() => page.evaluate(() => window.mode())).toBe("plan");
  await expect.poll(() => shown(page)).toEqual(["2"]);
});

test("a narrow pane steps the type down and hides nothing", async ({ page }) => {
  await open(page);
  const wide = await page.evaluate(() => window.displays());
  const wideHeading = await page.evaluate(() => window.headingSize());

  await page.setViewportSize({ width: 400, height: 520 });
  const narrow = await page.evaluate(() => window.displays());
  const narrowHeading = await page.evaluate(() => window.headingSize());

  expect(narrow).toHaveLength(wide.length);
  const lost = wide.filter((display, at) => display !== "none" && narrow[at] === "none");
  expect(lost).toEqual([]);
  expect(narrowHeading).toBeLessThan(wideHeading);
});
