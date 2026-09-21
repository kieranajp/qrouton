import { expect, test } from "@playwright/test";

const status = (page) => page.locator(".dialog .status");
const root = (page) => page.locator(".dialog input").nth(1);

test("a config that could not be read says so instead of an empty panel", async ({ page }) => {
  await page.goto("/tests/settings.html?fail=Load");
  await expect(page.locator(".dialog")).toBeVisible();

  await expect(status(page)).toContainText("Settings could not be read");
  await expect(status(page)).toContainText("permission denied");
  await expect(root(page)).toHaveValue("");
});

test("a config that answered fills the panel and says nothing", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await expect(root(page)).toHaveValue("/sessions");
  await expect(status(page)).toHaveText("");
});

test("sticker meanings load and save together without asking for a restart", async ({ page }) => {
  await page.goto("/tests/settings.html");
  const stickers = page.getByRole("group", { name: "Session stickers" });
  const star = stickers.getByRole("textbox", { name: "Blue star meaning" });
  const bookmark = stickers.getByRole("textbox", { name: "Green bookmark meaning" });
  const question = stickers.getByRole("textbox", { name: "Orange question mark meaning" });
  const exclamation = stickers.getByRole("textbox", { name: "Red exclamation mark meaning" });

  await expect(star).toHaveValue("Important");
  await expect(bookmark).toHaveValue("Read later");
  await expect(question).toHaveValue("Needs follow-up");
  await expect(exclamation).toHaveValue("Has bugs");

  await star.fill("Priority");
  await bookmark.fill("Read after launch");
  await question.fill("Needs an answer");
  await exclamation.fill("Broken here");
  await page.getByRole("button", { name: "Save" }).click();

  await expect.poll(() => page.evaluate(() => window.settingsFixture.saves())).toEqual([
    {
      vaultProfiles: [],
 vaultMappings: {},
 orgs: ["acme"],
      root: "/sessions",
      editor: "",
      launch: "",
      linear: "",
      stickerLabels: {
        star: "Priority",
        bookmark: "Read after launch",
        question: "Needs an answer",
        exclamation: "Broken here",
      },
    },
  ]);
  await expect(page.getByText("Quit qrouton to use the new sessions root")).toHaveCount(0);
});

test("a blank sticker meaning names the field and stays open", async ({ page }) => {
  await page.goto("/tests/settings.html?invalid=question");
  const question = page.getByRole("textbox", { name: "Orange question mark meaning" });
  await question.fill("   ");
  await page.getByRole("button", { name: "Save" }).click();

  await expect(status(page)).toHaveText("cannot be empty");
  await expect(
    question.locator(
      "xpath=ancestor::div[contains(concat(' ', normalize-space(@class), ' '), ' field ')][1]",
    ),
  ).toContainText("cannot be empty");
  await expect(page.locator(".dialog")).toBeVisible();
});


test("vault setup is optional and saves profiles with organisation mappings", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await expect(page.getByText("Select a read profile for this repository-free session.")).toHaveCount(0);
  await page.getByRole("button", { name: "Add vault", exact: true }).click();
  await page.getByRole("textbox", { name: "Vault name 1", exact: true }).fill("Shared");
  await page.getByRole("textbox", { name: "Vault folder 1", exact: true }).fill("/vault/shared");
  await page.getByRole("textbox", { name: "Vault organisation", exact: true }).fill("acme");
  await page.getByRole("combobox", { name: "Destination vault", exact: true }).selectOption({ label: "Shared" });
  await page.getByRole("button", { name: "Map organisation", exact: true }).click();
  await page.getByRole("button", { name: "Save", exact: true }).click();
  const saves = await page.evaluate(() => window.settingsFixture.saves());
  expect(saves[0].vaultProfiles[0]).toMatchObject({ name: "Shared", root: "/vault/shared" });
  expect(saves[0].vaultMappings.acme).toBe(saves[0].vaultProfiles[0].id);
});

test("vault validation stays visible without saving a relative root", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await page.getByRole("button", { name: "Add vault", exact: true }).click();
  await page.getByRole("textbox", { name: "Vault name 1", exact: true }).fill("Shared");
  await page.getByRole("textbox", { name: "Vault folder 1", exact: true }).fill("relative/vault");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(status(page)).toHaveText("invalid vault configuration");
  await expect(page.locator(".dialog")).toBeVisible();
});

test("a repository-free session requires explicit read selection", async ({ page }) => {
  await page.goto("/tests/settings.html?vaults");
  await expect(page.getByText("Select a read profile for this repository-free session.")).toBeVisible();
  await expect(page.getByRole("checkbox", { name: "Read Shared", exact: true })).not.toBeChecked();
  await page.getByRole("checkbox", { name: "Read Shared", exact: true }).check();
  await page.getByRole("textbox", { name: "Vault workstream", exact: true }).fill("storage");
  await page.getByRole("button", { name: "Save session scope", exact: true }).click();
  await expect(page.getByText("Session vault scope saved.")).toBeVisible();
  await expect(page.getByText(/Shared: 1 readable documents/)).toBeVisible();
  const scopes = await page.evaluate(() => window.settingsFixture.scopes());
  expect(scopes).toEqual([{ session: "example", workstream: "storage", selection: { readProfiles: ["shared"], repositories: [], destination: "", publicationDisabled: false } }]);
});


test("model download is explicit, reports progress, and cancels", async ({ page }) => {
  await page.goto("/tests/settings.html?vaults");
  await expect(page.getByRole("region", { name: "Vault indexing" })).toBeVisible();
  expect(await page.evaluate(() => window.settingsFixture.actions())).toEqual([]);
  await expect(page.getByText("4 readable; 1 invalid; 2 unsupported; 3 conflicts.")).toHaveCount(2);
  await page.getByRole("button", { name: "Download required model" }).click();
  await expect(page.getByRole("progressbar", { name: "Model download" })).toHaveAttribute("value", "4");
  await page.getByRole("button", { name: "Cancel download" }).click();
  await expect(page.getByText("Model download cancelled.")).toBeVisible();
  expect(await page.evaluate(() => window.settingsFixture.actions())).toEqual(["download", "cancel"]);
  await expect(page.getByText(/Exclude each vault’s/)).toBeVisible();
});

test("setup polling preserves unsaved session scope", async ({ page }) => {
  await page.goto("/tests/settings.html?vaults");
  await page.getByRole("checkbox", { name: "Read Shared", exact: true }).check();
  await page.getByRole("textbox", { name: "Vault workstream" }).fill("unsaved work");
  await expect.poll(() => page.evaluate(() => window.settingsFixture.setupReads())).toBeGreaterThan(1);
  await expect(page.getByRole("checkbox", { name: "Read Shared", exact: true })).toBeChecked();
  await expect(page.getByRole("textbox", { name: "Vault workstream" })).toHaveValue("unsaved work");
  await page.getByRole("button", { name: "Retry indexing Private", exact: true }).click();
  expect(await page.evaluate(() => window.settingsFixture.actions())).toEqual(["retry:private"]);
});

for (const item of [
  { query: "dependency=service_unavailable&not-installed", text: "Ollama was not found." },
  { query: "dependency=service_unavailable", text: "Ollama is stopped or unreachable." },
  { query: "dependency=incompatible", text: "The embedding model is incompatible." },
]) {
  test(`setup explains ${item.query}`, async ({ page }) => {
    await page.goto(`/tests/settings.html?vaults&${item.query}`);
    await expect(page.getByText(item.text, { exact: false })).toBeVisible();
    await expect(page.getByRole("button", { name: "Download required model" })).toHaveCount(0);
  });
}


test("setup reports its first failed status request", async ({ page }) => {
  await page.goto("/tests/settings.html?vaults&fail=VaultSetup");
  await expect(page.getByRole("status").filter({ hasText: "permission denied" })).toBeVisible();
});
