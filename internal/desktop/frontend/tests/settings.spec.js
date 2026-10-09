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
      chime: true,
      uiScale: 130,
      thoughts: {
        default: "",
        roots: [
          { id: "work", path: "/sync/work", orgs: "acme" },
          { id: "club", path: "/sync/club", orgs: "club, club-labs" },
        ],
      },
    },
  ]);
  await expect(page.getByText("Quit qrouton to use the new sessions root and thoughts folders")).toHaveCount(0);
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

test("the chime loads ticked and saves unticked", async ({ page }) => {
  await page.goto("/tests/settings.html");
  const chime = page.getByRole("checkbox", { name: "Chime when an agent is waiting for you" });
  await expect(chime).toBeChecked();

  await chime.uncheck();
  await page.getByRole("button", { name: "Save" }).click();

  await expect
    .poll(() => page.evaluate(() => window.settingsFixture.saves().map((save) => save.chime)))
    .toEqual([false]);
});

test("the UI scale loads its stored step and saves the chosen one", async ({ page }) => {
  await page.goto("/tests/settings.html");
  const scale = page.getByRole("combobox", { name: "UI scale" });
  await expect(scale).toHaveValue("130");
  await expect(scale.locator("option")).toHaveText(["80%", "90%", "100%", "110%", "120%", "130%", "140%", "150%"]);

  await scale.selectOption({ label: "150%" });
  await page.getByRole("button", { name: "Save" }).click();

  await expect
    .poll(() => page.evaluate(() => window.settingsFixture.saves().map((save) => save.uiScale)))
    .toEqual([150]);
});

test("the UI scale follows a change announced while the panel is open", async ({ page }) => {
  await page.goto("/tests/settings.html");
  const scale = page.getByRole("combobox", { name: "UI scale" });
  await expect(scale).toHaveValue("130");

  await page.evaluate(() => window.settingsFixture.announceScale(90));
  await expect(scale).toHaveValue("90");
});

const shared = (page) => page.getByRole("group", { name: "Shared thoughts folders" });
const warning = (page) => page.getByText("Sessions already using the old folder keep their documents");

test("shared folders load, add and remove, and save as rows", async ({ page }) => {
  await page.goto("/tests/settings.html");
  const names = shared(page).getByRole("textbox", { name: "Shared folder name" });
  await expect(names).toHaveCount(2);
  await expect(page.getByRole("textbox", { name: "Default thoughts folder" })).toHaveAttribute(
    "placeholder",
    "/sessions/thoughts",
  );

  await shared(page).getByRole("button", { name: "Add shared folder" }).click();
  await names.nth(2).fill("team");
  await shared(page).getByRole("textbox", { name: "Shared folder path" }).nth(2).fill("/sync/team");
  await shared(page).getByRole("textbox", { name: "Shared folder orgs" }).nth(2).fill("team-org");
  await shared(page).getByRole("button", { name: "Remove club" }).click();
  await expect(warning(page)).toBeVisible();
  await page.getByRole("button", { name: "Save" }).click();

  await expect
    .poll(() => page.evaluate(() => window.settingsFixture.saves().map((save) => save.thoughts)))
    .toEqual([
      {
        default: "",
        roots: [
          { id: "work", path: "/sync/work", orgs: "acme" },
          { id: "team", path: "/sync/team", orgs: "team-org" },
        ],
      },
    ]);
});

test("a saved folder's name is read-only and a new one's is not", async ({ page }) => {
  await page.goto("/tests/settings.html");
  const names = shared(page).getByRole("textbox", { name: "Shared folder name" });
  await expect(names.first()).toHaveAttribute("readonly", "");
  await shared(page).getByRole("button", { name: "Add shared folder" }).click();
  await expect(names.nth(2)).toBeEditable();
});

test("a changed path warns that nothing moves, and changed orgs do not", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await expect(shared(page).getByRole("textbox", { name: "Shared folder name" })).toHaveCount(2);
  await shared(page).getByRole("textbox", { name: "Shared folder orgs" }).first().fill("acme, more");
  await expect(warning(page)).toHaveCount(0);

  await shared(page).getByRole("textbox", { name: "Shared folder path" }).first().fill("/sync/new");
  await expect(warning(page)).toBeVisible();
});

test("a thoughts refusal shows under the section and keeps the panel open", async ({ page }) => {
  await page.goto("/tests/settings.html?invalid=thoughts");
  await expect(shared(page).getByRole("textbox", { name: "Shared folder name" })).toHaveCount(2);
  await page.getByRole("button", { name: "Save" }).click();

  await expect(page.locator(".note.failed")).toHaveText("an org maps to two thoughts roots");
  await expect(status(page)).toHaveText("an org maps to two thoughts roots");
  await expect(page.locator(".dialog")).toBeVisible();
});
