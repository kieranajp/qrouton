import { expect, test } from "@playwright/test";

test("every tag is a filled block of its kind's own hue", async ({ page }) => {
  await page.goto("/tests/artifact-picker.html");
  const tags = await page.locator(".tag").evaluateAll((found) =>
    found.map((tag) => ({
      label: tag.textContent,
      colour: getComputedStyle(tag).backgroundColor,
      ink: getComputedStyle(tag).color,
    })),
  );

  const CRUST = "rgb(24, 25, 38)";
  expect(tags).toEqual([
    { label: "P1", colour: "rgb(183, 189, 248)", ink: CRUST },
    { label: "S1", colour: "rgb(245, 189, 230)", ink: CRUST },
    { label: "R1", colour: "rgb(145, 215, 227)", ink: CRUST },
    { label: "NOTE", colour: "rgb(240, 198, 198)", ink: CRUST },
    { label: "P1", colour: "rgb(183, 189, 248)", ink: CRUST },
    { label: "R1", colour: "rgb(145, 215, 227)", ink: CRUST },
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

  await expect(page.getByRole("button", { name: /Documents 0/ })).toBeVisible();
  await expect(page.locator(".heading")).toHaveText(["Written this session", "In-repo"]);
  await expect(page.getByRole("button", { name: "qrouton" })).toBeVisible();
});
