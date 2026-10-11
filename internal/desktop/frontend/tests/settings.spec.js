import { expect, test } from "@playwright/test";

const status = (page) => page.locator(".dialog .status");
const tab = (page, name) => page.getByRole("tab", { name });
const open = async (page, name) => {
  await tab(page, name).click();
  await expect(tab(page, name)).toHaveAttribute("aria-selected", "true");
};
const root = (page) => page.getByRole("textbox", { name: "Sessions root" });
const save = (page) => page.getByRole("button", { name: "Save" }).click();
const saves = (page) => page.evaluate(() => window.settingsFixture.saves());

test("a config that could not be read says so instead of an empty panel", async ({ page }) => {
  await page.goto("/tests/settings.html?fail=Load");
  await expect(page.locator(".dialog")).toBeVisible();

  await expect(status(page)).toContainText("Settings could not be read");
  await expect(status(page)).toContainText("permission denied");
  await open(page, "Folders");
  await expect(root(page)).toHaveValue("");
});

test("a config that answered fills the panel and says nothing", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await open(page, "Folders");
  await expect(root(page)).toHaveValue("/sessions");
  await expect(status(page)).toHaveText("");
});

test("settings opens on General with six tabs in index order", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await expect(page.getByRole("tab")).toHaveText(["General", "GitHub", "Folders", "Search", "Stickers", "Advanced"]);
  await expect(tab(page, "General")).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("tabpanel")).toContainText("Editor command");
  await expect(page.getByText("Written to config.json")).toBeVisible();
});

test("arrow keys, Home and End move through the tabs and take focus with them", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await tab(page, "General").focus();
  await page.keyboard.press("ArrowDown");
  await expect(tab(page, "GitHub")).toBeFocused();
  await expect(tab(page, "GitHub")).toHaveAttribute("aria-selected", "true");
  await page.keyboard.press("End");
  await expect(tab(page, "Advanced")).toBeFocused();
  await page.keyboard.press("ArrowDown");
  await expect(tab(page, "General")).toBeFocused();
  await page.keyboard.press("ArrowUp");
  await expect(tab(page, "Advanced")).toBeFocused();
  await page.keyboard.press("Home");
  await expect(tab(page, "General")).toHaveAttribute("aria-selected", "true");
  await expect(tab(page, "GitHub")).toHaveAttribute("tabindex", "-1");
});

test("sticker meanings load and save together without asking for a restart", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await open(page, "Stickers");
  const stickers = page.getByRole("list", { name: "Session stickers" });
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
  await save(page);

  await expect.poll(() => saves(page)).toEqual([
    {
      orgs: ["acme"],
      root: "/sessions",
      editor: "",
      launch: "",
      stickers: [
        { id: "star", colour: "blue", symbol: "star", label: "Priority" },
        { id: "bookmark", colour: "green", symbol: "bookmark", label: "Read after launch" },
        { id: "question", colour: "orange", symbol: "question", label: "Needs an answer" },
        { id: "exclamation", colour: "red", symbol: "exclamation", label: "Broken here" },
      ],
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
  await expect(page.getByText("Saved. Quit and restart to use the new folders.")).toHaveCount(0);
});

test("a refused field switches to its tab, marks it, and keeps the panel open", async ({ page }) => {
  await page.goto("/tests/settings.html?invalid=stickers");
  await open(page, "Stickers");
  await page.getByRole("textbox", { name: "Orange question mark meaning" }).fill("   ");
  await open(page, "General");
  await save(page);

  await expect(tab(page, "Stickers")).toHaveAttribute("aria-selected", "true");
  await expect(tab(page, "Stickers")).toHaveAccessibleName("Stickers has an error");
  await expect(status(page)).toHaveText("cannot be empty");
  await expect(page.locator(".row").nth(2)).toContainText("cannot be empty");
  await expect(page.locator(".dialog")).toBeVisible();
});

test("the restart footer appears only after a save that needs a restart", async ({ page }) => {
  await page.goto("/tests/settings.html?restart=1");
  const footer = page.getByText("Saved. Quit and restart to use the new folders.");
  await open(page, "Folders");
  await expect(footer).toHaveCount(0);
  await open(page, "General");
  await save(page);
  await expect(footer).toBeVisible();
  await page.getByRole("button", { name: "Quit and restart" }).click();
  await expect.poll(() => page.evaluate(() => window.settingsFixture.calls())).toContain("Relaunch");
});

test("an editor preset fills the field and shows as chosen", async ({ page }) => {
  await page.goto("/tests/settings.html");
  const presets = page.getByRole("group", { name: "Start from" }).getByRole("button");
  await expect(presets).toHaveText([
    "nvim +{line} {path}",
    "zed {path}:{line}",
    "code -g {path}:{line}",
    "cursor -g {path}:{line}",
    "subl {path}:{line}",
  ]);
  await page.getByRole("button", { name: "zed {path}:{line}" }).click();
  await expect(page.getByRole("textbox", { name: "Editor command" })).toHaveValue("zed {path}:{line}");
  await expect(page.getByRole("button", { name: "zed {path}:{line}" })).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("textbox", { name: "Editor command" }).fill("zed --wait {path}");
  await expect(page.getByRole("button", { name: "zed {path}:{line}" })).toHaveAttribute("aria-pressed", "false");
});

test("the chime loads ticked and saves unticked", async ({ page }) => {
  await page.goto("/tests/settings.html");
  const chime = page.getByRole("checkbox", { name: "Chime when an agent is waiting for you" });
  await expect(chime).toBeChecked();
  await expect(page.getByText("Stays quiet for the session you're looking at.")).toBeVisible();

  await chime.uncheck();
  await save(page);

  await expect.poll(async () => (await saves(page)).map((saved) => saved.chime)).toEqual([false]);
});

test("the UI scale offers all eight steps and saves the chosen one", async ({ page }) => {
  await page.goto("/tests/settings.html");
  const scale = page.getByRole("group", { name: "UI scale" }).getByRole("button");
  await expect(scale).toHaveText(["80%", "90%", "100%", "110%", "120%", "130%", "140%", "150%"]);
  await expect(page.getByRole("button", { name: "130%" })).toHaveAttribute("aria-pressed", "true");

  await page.getByRole("button", { name: "150%" }).click();
  await save(page);

  await expect.poll(async () => (await saves(page)).map((saved) => saved.uiScale)).toEqual([150]);
});

test("the UI scale follows a change announced while the panel is open", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await expect(page.getByRole("button", { name: "130%" })).toHaveAttribute("aria-pressed", "true");

  await page.evaluate(() => window.settingsFixture.announceScale(90));
  await expect(page.getByRole("button", { name: "90%" })).toHaveAttribute("aria-pressed", "true");
});

const shared = (page) => page.getByRole("group", { name: "Shared thoughts folders" });
const warning = (page) => page.getByText("Sessions already using the old folder keep their documents");

test("shared folders load, add and remove, and save as rows", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await open(page, "Folders");
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
  await save(page);

  await expect.poll(async () => (await saves(page)).map((saved) => saved.thoughts)).toEqual([
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
  await open(page, "Folders");
  const names = shared(page).getByRole("textbox", { name: "Shared folder name" });
  await expect(names.first()).toHaveAttribute("readonly", "");
  await shared(page).getByRole("button", { name: "Add shared folder" }).click();
  await expect(names.nth(2)).toBeEditable();
});

test("a changed path warns that nothing moves, and changed orgs do not", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await open(page, "Folders");
  await expect(shared(page).getByRole("textbox", { name: "Shared folder name" })).toHaveCount(2);
  await shared(page).getByRole("textbox", { name: "Shared folder orgs" }).first().fill("acme, more");
  await expect(warning(page)).toHaveCount(0);

  await shared(page).getByRole("textbox", { name: "Shared folder path" }).first().fill("/sync/new");
  await expect(warning(page)).toBeVisible();
});

test("a thoughts refusal switches to Folders and shows under the section", async ({ page }) => {
  await page.goto("/tests/settings.html?invalid=thoughts");
  await save(page);

  await expect(tab(page, "Folders")).toHaveAttribute("aria-selected", "true");
  await expect(page.locator(".note.failed")).toHaveText("an org maps to two thoughts roots");
  await expect(status(page)).toHaveText("an org maps to two thoughts roots");
  await expect(page.locator(".dialog")).toBeVisible();
});

test("a root pinned by QROUTON_ROOT says the file's value is what Save writes", async ({ page }) => {
  await page.goto("/tests/settings.html?pinned=root");
  await open(page, "Folders");
  await expect(root(page)).toHaveValue("/sessions");
  await expect(page.locator(".dialog")).toContainText("Set by QROUTON_ROOT for this run. Save writes the file's value.");
  await open(page, "GitHub");
  await expect(page.locator(".dialog")).not.toContainText("QROUTON_ORGS");
});

test("orgs add and remove as rows", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await open(page, "GitHub");
  await page.getByRole("textbox", { name: "Orgs" }).fill("second-org");
  await page.getByRole("button", { name: "Add" }).click();
  await page.getByRole("button", { name: "Remove acme" }).click();
  await save(page);
  await expect.poll(async () => (await saves(page)).map((saved) => saved.orgs)).toEqual([["second-org"]]);
});

test("Search draws a missing Ollama with a copyable install command and the download link", async ({ page }) => {
  await page.goto("/tests/settings.html?ollama=unreachable");
  await open(page, "Search");
  await expect(page.getByText("Ollama isn't installed")).toBeVisible();
  await page.getByRole("button", { name: "Copy brew install ollama" }).click();
  await expect.poll(() => page.evaluate(() => window.clipboardText)).toBe("brew install ollama");
  await page.getByRole("link", { name: "ollama.com/download" }).click();
  await expect.poll(() => page.evaluate(() => window.openedURL)).toBe("https://ollama.com/download");
  await page.getByRole("button", { name: "Check again" }).click();
  await expect.poll(async () => (await page.evaluate(() => window.settingsFixture.calls())).filter((c) => c === "Check").length).toBe(2);
});

test("Search tells a stopped Ollama apart from a missing one", async ({ page }) => {
  await page.goto("/tests/settings.html?ollama=unreachable&installed=1");
  await open(page, "Search");
  await expect(page.getByText("Ollama is installed but not running")).toBeVisible();
  await expect(page.getByText("ollama serve")).toBeVisible();
});

test("Search says when semantic search is ready", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await open(page, "Search");
  await expect(page.getByText("Semantic search is ready.")).toBeVisible();
});

test("Advanced lists every runner id the launch overrides accept", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await open(page, "Advanced");
  await expect(page.locator(".legend .term")).toHaveText(["claude", "codex", "opencode", "agy"]);
  await expect(page.getByRole("textbox", { name: "Launch overrides" })).toBeVisible();
  await expect(page.getByRole("textbox", { name: "Linear custom script" })).toHaveCount(0);
});

const stickerRows = (page) => page.getByRole("list", { name: "Session stickers" }).getByRole("listitem");
const savedStickers = async (page) => (await saves(page)).map((saved) => saved.stickers);

test("a new sticker takes the first unused pair, can be recoloured and saves without an id", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await open(page, "Stickers");
  await page.getByRole("button", { name: "Add sticker" }).click();
  await expect(stickerRows(page)).toHaveCount(5);
  const meaning = page.getByRole("textbox", { name: "Sapphire star meaning" });
  await expect(meaning).toBeFocused();
  await meaning.fill("Ship it");
  await page.getByRole("group", { name: "Colour" }).getByRole("button", { name: "pink" }).click();
  await page.getByRole("group", { name: "Symbol" }).getByRole("button", { name: "heart" }).click();
  await expect(page.getByRole("textbox", { name: "Pink heart meaning" })).toHaveValue("Ship it");
  await expect(page.getByRole("group", { name: "Edit sticker" })).toContainText("Pink heart");
  await save(page);

  await expect.poll(async () => (await savedStickers(page))[0]?.at(-1)).toEqual({
    id: "",
    colour: "pink",
    symbol: "heart",
    label: "Ship it",
  });
});

test("removing a sticker drops it from the save", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await open(page, "Stickers");
  await page.getByRole("button", { name: "Remove Read later" }).click();
  await expect(stickerRows(page)).toHaveCount(3);
  await save(page);
  await expect
    .poll(async () => (await savedStickers(page))[0]?.map((item) => item.id))
    .toEqual(["star", "question", "exclamation"]);
});

test("a duplicate pair is flagged on its row before Save and refused after it", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await open(page, "Stickers");
  await page.getByRole("button", { name: "Edit Green bookmark" }).click();
  await page.getByRole("group", { name: "Colour" }).getByRole("button", { name: "blue" }).click();
  await page.getByRole("group", { name: "Symbol" }).getByRole("button", { name: "star", exact: true }).click();
  await expect(stickerRows(page).nth(1)).toContainText("another sticker already uses this colour and symbol");
  await save(page);
  await expect(status(page)).toHaveText("another sticker already uses this colour and symbol");
  await expect(stickerRows(page).nth(1)).toContainText("another sticker already uses this colour and symbol");
  await expect(page.locator(".dialog")).toBeVisible();
});

test("an empty sticker list saves as empty", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await open(page, "Stickers");
  for (const label of ["Important", "Read later", "Needs follow-up", "Has bugs"]) {
    await page.getByRole("button", { name: `Remove ${label}` }).click();
  }
  await expect(stickerRows(page)).toHaveCount(0);
  await expect(page.getByRole("group", { name: "Edit sticker" })).toHaveCount(0);
  await save(page);
  await expect.poll(() => savedStickers(page)).toEqual([[]]);
});

const gitHubChecks = async (page) =>
  (await page.evaluate(() => window.settingsFixture.calls())).filter((call) => call === "GitHubStatus").length;

test("GitHub status is checked when its tab first opens, not before", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await expect(tab(page, "General")).toHaveAttribute("aria-selected", "true");
  expect(await gitHubChecks(page)).toBe(0);
  await open(page, "GitHub");
  await expect(page.getByText("Signed in as kieranajp via the GitHub CLI")).toBeVisible();
  await open(page, "General");
  await open(page, "GitHub");
  expect(await gitHubChecks(page)).toBe(1);
});

test("an environment token names its variable", async ({ page }) => {
  await page.goto("/tests/settings.html?gh=env");
  await open(page, "GitHub");
  await expect(page.getByText("Signed in as kieranajp via GITHUB_TOKEN")).toBeVisible();
});

test("a missing GitHub CLI offers the install commands, the site and Check again", async ({ page }) => {
  await page.goto("/tests/settings.html?gh=missing");
  await open(page, "GitHub");
  await expect(page.getByText("The GitHub CLI isn't installed")).toBeVisible();
  await page.getByRole("button", { name: "Copy brew install gh" }).click();
  await expect.poll(() => page.evaluate(() => window.clipboardText)).toBe("brew install gh");
  await expect(page.getByRole("button", { name: "Copy gh auth login" })).toBeVisible();
  await page.getByRole("link", { name: "cli.github.com" }).click();
  await expect.poll(() => page.evaluate(() => window.openedURL)).toBe("https://cli.github.com");
  await page.getByRole("button", { name: "Check again" }).click();
  await expect.poll(() => gitHubChecks(page)).toBe(2);
});

test("a signed-out GitHub CLI asks for gh auth login", async ({ page }) => {
  await page.goto("/tests/settings.html?gh=signed-out");
  await open(page, "GitHub");
  await expect(page.getByText("The GitHub CLI isn't signed in")).toBeVisible();
  await expect(page.getByRole("button", { name: "Copy gh auth login" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Copy brew install gh" })).toHaveCount(0);
});

test("a missing shared folder is flagged until its path is edited", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await open(page, "Folders");
  const flags = page.getByText("· missing");
  await expect(flags).toHaveCount(1);
  await expect(shared(page).locator(".cell.path").nth(1)).toContainText("· missing");
  await shared(page).getByRole("textbox", { name: "Shared folder path" }).nth(1).fill("/sync/club-now");
  await expect(flags).toHaveCount(0);
});

const calls = (page) => page.evaluate(() => window.settingsFixture.calls());

test("Install Linear integration runs at once and Save carries no Linear text", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await open(page, "Advanced");
  await expect(page.getByLabel("Linear entry", { exact: true })).toContainText("{{issue.identifier}}");
  await page.getByRole("button", { name: "Install Linear integration" }).click();
  await expect(page.getByText("Installed in ~/.linear/coding-tools.json")).toBeVisible();
  expect(await calls(page)).toContain("InstallLinear");
  await page.getByRole("button", { name: "Copy the Linear entry" }).click();
  await expect.poll(() => page.evaluate(() => window.clipboardText)).toContain('"openIssue"');
  await save(page);
  await expect.poll(async () => (await saves(page)).length).toBe(1);
  expect(Object.keys((await saves(page))[0])).not.toContain("linear");
});

test("Install says it replaces an openIssue entry that is already there", async ({ page }) => {
  await page.goto("/tests/settings.html?linear=replaces");
  await open(page, "Advanced");
  await expect(
    page.getByRole("button", { name: "Install Linear integration, replacing the current entry" }),
  ).toBeVisible();
  await expect(page.getByText("already has a different openIssue entry")).toBeVisible();
});

test("Open config.json asks the workbench to open the file", async ({ page }) => {
  await page.goto("/tests/settings.html");
  await open(page, "Advanced");
  await expect(page.getByText("Opens in your default text editor. Every setting lives here.")).toBeVisible();
  await page.getByRole("button", { name: "Open config.json" }).click();
  await expect.poll(() => calls(page)).toContain("OpenConfig");
});

const MIN_FIELD_WIDTH = 120;

const atLargestScale = async (page) => {
  await page.setViewportSize({ width: 1100, height: 760 });
  await page.goto("/tests/settings.html");
  await page.evaluate(() => document.documentElement.style.setProperty("--ui-scale", "1.5"));
};

test("at 150% in the default window a sticker meaning stays wide enough to type in", async ({ page }) => {
  await atLargestScale(page);
  await open(page, "Stickers");
  await page.getByRole("button", { name: "Add sticker" }).click();
  const meaning = page.getByRole("textbox", { name: "Sapphire star meaning" });
  expect((await meaning.boundingBox()).width).toBeGreaterThanOrEqual(MIN_FIELD_WIDTH);
  await meaning.fill("Waiting on review");
  await expect(meaning).toHaveValue("Waiting on review");
});

test("at 150% in the default window no tab collapses a text field", async ({ page }) => {
  await atLargestScale(page);
  const measured = [];
  for (const name of ["General", "GitHub", "Folders", "Search", "Stickers", "Advanced"]) {
    await open(page, name);
    const fields = page.getByRole("tabpanel").locator("input:not([type=checkbox]):not([type=radio]), textarea");
    for (const field of await fields.all()) {
      measured.push(name);
      const box = await field.boundingBox();
      const label = (await field.getAttribute("aria-label")) ?? (await field.getAttribute("placeholder"));
      expect(box?.width ?? 0, `${name}: ${label}`).toBeGreaterThanOrEqual(MIN_FIELD_WIDTH);
    }
  }
  expect(new Set(measured)).toEqual(new Set(["General", "GitHub", "Folders", "Stickers", "Advanced"]));
});
