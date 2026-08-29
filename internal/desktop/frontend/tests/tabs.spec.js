import { expect, test } from "@playwright/test";

const PLAN_LAVENDER = "rgb(183, 189, 248)";

const open = async (page, query = "") => {
  await page.goto("/tests/tabs.html" + query);
  await page.waitForSelector(".tab");
};

test("a plan tab leads with its id in the plan colour", async ({ page }) => {
  await open(page);

  await expect.poll(() => page.evaluate(() => window.labels())).toEqual([
    { text: "Shell", title: "Shell", badge: "", badgeColour: "" },
    {
      text: "[P002]Pane smoke test",
      title: "[P002] Pane smoke test",
      badge: "[P002]",
      badgeColour: PLAN_LAVENDER,
    },
    { text: "◆ Findings", title: "◆ Findings", badge: "", badgeColour: "" },
  ]);
});

test("a tab the strip cannot fit keeps its id in the overflow menu", async ({ page }) => {
  await open(page, "?narrow");

  await page.getByRole("button", { name: /more tabs/ }).click();
  await expect.poll(() => page.evaluate(() => window.menuLabels())).toContain(
    "[P002] Pane smoke test",
  );
});
