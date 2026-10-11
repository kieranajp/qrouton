import { expect, test } from "@playwright/test";

const PLAN_LAVENDER = "rgb(183, 189, 248)";
const RESEARCH_SKY = "rgb(145, 215, 227)";
const RUNNING_TEAL = "rgb(139, 213, 202)";
const SUCCESS_GREEN = "rgb(166, 218, 149)";

const open = async (page, query = "") => {
  await page.goto("/tests/tabs.html" + query);
  await page.waitForSelector(".tab");
};

const titles = (page) => page.evaluate(() => window.labels().map((tab) => tab.text));

test("a badged tab leads with its id in a filled block of its own hue", async ({ page }) => {
  await open(page);

  await expect.poll(() => page.evaluate(() => window.labels())).toEqual([
    { text: "Overview", title: "Overview", badge: "", badgeColour: "" },
    { text: "Shell", title: "Shell", badge: "", badgeColour: "" },
    {
      text: "Pane smoke test",
      title: "P002 Pane smoke test",
      badge: "P002",
      badgeColour: PLAN_LAVENDER,
    },
    {
      text: "Pane selection",
      title: "R001 Pane selection",
      badge: "R001",
      badgeColour: RESEARCH_SKY,
    },
    { text: "◆ Findings", title: "◆ Findings", badge: "", badgeColour: "" },
  ]);
});

test("a tab the strip cannot fit keeps its id in the overflow menu", async ({ page }) => {
  await open(page, "?narrow");

  await page.getByRole("button", { name: /more tabs/ }).click();
  await expect.poll(() => page.evaluate(() => window.menuLabels())).toContain(
    "P002 Pane smoke test",
  );
});

test("a finished run is green under the word the workbench sends for it", async ({ page }) => {
  await open(page);

  await expect.poll(() => page.evaluate(() => window.dots())).toEqual([
    "",
    RUNNING_TEAL,
    "",
    "",
    SUCCESS_GREEN,
  ]);
});

test("the overflow menu keeps a hidden run's colour", async ({ page }) => {
  await open(page, "?narrow");

  await page.getByRole("button", { name: /more tabs/ }).click();
  await expect.poll(() => page.evaluate(() => window.menuDots())).toContain(SUCCESS_GREEN);
});

test("a tab dragged onto another takes the place it was dropped on", async ({ page }) => {
  await open(page);

  await page.locator(".tab").nth(1).dragTo(page.locator(".tab").nth(3));

  await expect
    .poll(() => titles(page))
    .toEqual(["Overview", "Pane smoke test", "Pane selection", "Shell", "◆ Findings"]);
});

test("the middle mouse button closes a tab", async ({ page }) => {
  await open(page);

  await page.locator(".tab").nth(2).click({ button: "middle" });

  await expect.poll(() => titles(page)).toEqual(["Overview", "Shell", "Pane selection", "◆ Findings"]);
});

test("the pinned Overview tab cannot be closed or moved", async ({ page }) => {
  await open(page);
  const overview = page.locator('.tab[title="Overview"]');

  await expect(overview.getByRole("button", { name: "Close tab" })).toHaveCount(0);
  await expect(overview).toHaveAttribute("draggable", "false");
  await overview.click({ button: "middle" });
  await page.locator(".tab").nth(2).dragTo(overview);

  await expect
    .poll(() => titles(page))
    .toEqual(["Overview", "Shell", "Pane smoke test", "Pane selection", "◆ Findings"]);
});

// The Overview is the session's home, not a document: sized to its label,
// named in caps, and closed off from the other tabs by a heavier divider.
test("the Overview tab gets its own treatment", async ({ page }) => {
  await open(page);
  const home = page.locator('.tab[title="Overview"]');
  const style = await home.evaluate((el) => {
    const tab = getComputedStyle(el);
    const label = getComputedStyle(el.querySelector(".label"));
    return {
      grow: tab.flexGrow,
      divider: tab.borderRightWidth,
      caps: label.textTransform,
      glyph: Boolean(el.querySelector(".glyph")),
      width: el.getBoundingClientRect().width,
    };
  });
  expect(style).toMatchObject({ grow: "0", divider: "3px", caps: "uppercase", glyph: true });
  const shell = await page.locator('.tab[title="Shell"]').evaluate((el) => getComputedStyle(el).borderRightWidth);
  expect(shell).toBe("1px");
  expect(style.width).toBeLessThan(140);
});
