import { expect, test } from "@playwright/test";

const heading = (page) => page.locator("h1");
// Enter is the dialog's own key, and the layer is what holds the keyboard until
// a field takes it.
const advance = (page) => page.locator(".layer").press("Enter");

const toOwners = async (page, query = "") => {
  await page.goto("/tests/firstrun.html" + query);
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
  await expect(heading(page)).toHaveText("Where should thoughts go?");

  await advance(page);
  await expect(heading(page)).toHaveText("Search by meaning as well?");
  await expect(page.getByText("search will be keyword-only")).toBeVisible();

  await advance(page);
  await expect.poll(() => page.evaluate(() => window.saves)).toEqual([
    { orgs: ["acme"], root: "/sessions", thoughts: "/sessions/thoughts" },
  ]);
});

const toRoot = async (page, query = "") => {
  await toOwners(page, query);
  const field = page.getByPlaceholder("Add an org or username…");
  await field.fill("acme");
  await field.press("Enter");
  await advance(page);
  await expect(heading(page)).toHaveText("Where should sessions live?");
};

const field = (page) => page.locator(".dialog input").first();
const forward = (page, name) => page.getByRole("button", { name }).click();

test("the thoughts folder follows the root until the user types one", async ({ page }) => {
  await toRoot(page);
  await forward(page, "Next →");
  await expect(field(page)).toHaveValue("/sessions/thoughts");

  await forward(page, "← Back");
  await field(page).fill("/work/");
  await forward(page, "Next →");
  await expect(field(page)).toHaveValue("/work/thoughts");

  await field(page).fill("/vaults/mine");
  await forward(page, "← Back");
  await field(page).fill("/elsewhere");
  await forward(page, "Next →");
  await expect(field(page)).toHaveValue("/vaults/mine");

  await forward(page, "Next →");
  await page.evaluate(() =>
    window.emitOllama({ state: "pulling", model: "all-minilm", completed: 1, total: 4 }),
  );
  await expect(page.getByRole("button", { name: "Cancel" })).toBeVisible();
  await forward(page, "Find my repositories →");
  await expect.poll(() => page.evaluate(() => window.saves)).toEqual([
    { orgs: ["acme"], root: "/elsewhere", thoughts: "/vaults/mine" },
  ]);
});

test("a refused root returns to the root screen", async ({ page }) => {
  await toRoot(page, "?refuse=root");
  await forward(page, "Next →");
  await forward(page, "Next →");
  await forward(page, "Find my repositories →");

  await expect(heading(page)).toHaveText("Where should sessions live?");
  await expect(page.locator(".help.failed")).toHaveText("refused");
});
