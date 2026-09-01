import { expect, test } from "@playwright/test";

const PLAN_LAVENDER = "rgb(183, 189, 248)";
const RESEARCH_SKY = "rgb(145, 215, 227)";
const RUNNING_TEAL = "rgb(139, 213, 202)";
const SUCCESS_GREEN = "rgb(166, 218, 149)";

const open = async (page, query = "") => {
  await page.goto("/tests/tabs.html" + query);
  await page.waitForSelector(".tab");
};

test("a badged tab leads with its id in a filled block of its own hue", async ({ page }) => {
  await open(page);

  await expect.poll(() => page.evaluate(() => window.labels())).toEqual([
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
