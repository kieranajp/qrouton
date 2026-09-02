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
