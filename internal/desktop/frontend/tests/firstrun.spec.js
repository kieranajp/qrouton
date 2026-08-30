import { expect, test } from "@playwright/test";

const heading = (page) => page.locator("h1");
// Enter is the dialog's own key, and the layer is what holds the keyboard until
// a field takes it.
const advance = (page) => page.locator(".layer").press("Enter");

const toOwners = async (page) => {
  await page.goto("/tests/firstrun.html");
  await expect(heading(page)).toHaveText("qrouton");
  await advance(page);
  await advance(page);
  await advance(page);
  await expect(heading(page)).toHaveText("Whose repositories should I search?");
};

test("Enter does not carry an unanswered question forward", async ({ page }) => {
  await toOwners(page);

  await expect(page.getByRole("button", { name: "Next →" })).toBeDisabled();
  await expect(page.locator(".status")).toHaveText(
    "Add at least one organisation or username to search.",
  );

  await advance(page);
  await expect(heading(page)).toHaveText("Whose repositories should I search?");
});

test("an answered question advances on the same key", async ({ page }) => {
  await toOwners(page);

  const field = page.getByPlaceholder("Add an org or username…");
  await field.fill("acme");
  await field.press("Enter");
  await expect(page.getByRole("button", { name: "Next →" })).toBeEnabled();
  await expect(heading(page)).toHaveText("Whose repositories should I search?");

  await advance(page);
  await expect(heading(page)).toHaveText("Where should sessions live?");

  await advance(page);
  await expect.poll(() => page.evaluate(() => window.saves)).toEqual([
    { orgs: ["acme"], root: "/sessions" },
  ]);
});
