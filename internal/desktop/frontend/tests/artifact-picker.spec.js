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
    { kind: "PLAN", colour: "rgb(183, 189, 248)" },
    { kind: "RESEARCH", colour: "rgb(145, 215, 227)" },
  ]);
});

test("the artifact picker groups repository thoughts in a nested menu", async ({ page }) => {
  await page.goto("/tests/artifact-picker.html");

  await expect(page.locator(".heading")).toHaveText(["Written this session", "In-repo"]);
  const repository = page.getByRole("button", { name: "qrouton" });
  await expect(repository).toHaveAttribute("aria-haspopup", "menu");
  await expect(page.getByRole("button", { name: /plans\/P1-in-repo\.md/ })).toBeHidden();

  await repository.hover();
  const plan = page.getByRole("button", { name: /plans\/P1-in-repo\.md/ });
  await expect(plan).toBeVisible();
  await plan.click();
  await expect.poll(() => page.evaluate(() => window.selectedArtifact)).toBe(
    "src/qrouton/thoughts/plans/P1-in-repo.md",
  );
});

test("repository thoughts remain available before the session writes anything", async ({ page }) => {
  await page.goto("/tests/artifact-picker.html?repository-only");

  await expect(page.getByRole("button", { name: /nothing yet/ })).toBeVisible();
  await expect(page.locator(".heading")).toHaveText(["Written this session", "In-repo"]);
  await expect(page.getByRole("button", { name: "qrouton" })).toBeVisible();
});
