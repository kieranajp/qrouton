import { expect, test } from "@playwright/test";

test("the artifact picker uses the design token for every kind", async ({ page }) => {
  await page.goto("/tests/artifact-picker.html");
  const colours = await page.locator(".tag").evaluateAll((tags) =>
    tags.map((tag) => ({
      kind: tag.textContent,
      colour: getComputedStyle(tag).backgroundColor,
    })),
  );

  expect(colours).toEqual([
    { kind: "PLAN", colour: "rgb(183, 189, 248)" },
    { kind: "PLAN", colour: "rgb(183, 189, 248)" },
    { kind: "SPEC", colour: "rgb(245, 189, 230)" },
    { kind: "RESEARCH", colour: "rgb(145, 215, 227)" },
    { kind: "NOTE", colour: "rgb(240, 198, 198)" },
  ]);
});
