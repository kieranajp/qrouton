import { expect, test } from "@playwright/test";
import { ASSEMBLY_PROGRESS_EVENT, PROFILES_STATUS, REPOS_REFRESH_EVENT, SIGN_IN_STATES } from "../src/lib/bridge/generated.js";

test("clearing repository searches keeps selected rows first, ahead of the display cap", async ({ page }) => {
  await page.goto("/tests/assembly.html");
  await page.waitForFunction(() => window.assembly?.calls().some(({ name }) => name.endsWith(".Begin")));
  await page.evaluate(() => window.assembly.resolveBegin({ ticket: "", entropy: "4f3a", generation: 7 }));
  await page.getByRole("button", { name: "Choose repositories" }).click();
  await page.evaluate((event) => window.assembly.emit(event, {
    generation: 1,
    repos: Array.from({ length: 60 }, (_, i) => ({ org: "acme", name: `repo${String(i).padStart(2, "0")}`, default_branch: "main" })),
  }), REPOS_REFRESH_EVENT);

  const search = page.getByRole("textbox", { name: "Search repositories" });
  const clear = page.getByRole("button", { name: "Clear repository search" });
  const names = page.locator(".rows .name");
  await expect(names).toHaveCount(50);
  await expect(clear).toHaveCount(0);
  await search.fill("repo59");
  await expect(names).toHaveText(["acme/repo59"]);
  await page.locator(".rows").getByRole("button", { name: "Editing", exact: true }).click();
  await clear.click();

  await expect(search).toHaveValue("");
  await expect(search).toBeFocused();
  await expect(clear).toHaveCount(0);
  await expect(names).toHaveCount(50);
  await expect(names.first()).toHaveText("acme/repo59");
  await expect(page.locator(".rows .chosen").getByRole("button", { name: "Editing", exact: true })).toHaveAttribute("aria-pressed", "true");

  await search.fill("repo58");
  await page.locator(".rows").getByRole("button", { name: "Reference", exact: true }).click();
  await search.focus();
  await search.press("Tab");
  await expect(clear).toBeFocused();
  await clear.press("Enter");
  await expect(search).toBeFocused();
  await expect(search).toHaveValue("");
  await expect(names.nth(0)).toHaveText("acme/repo58");
  await expect(names.nth(1)).toHaveText("acme/repo59");
  await expect(page.locator(".roles")).toHaveText("1 editing · 1 reference");

  await page.locator(".rows .row").first().getByRole("button", { name: "Off", exact: true }).click();
  await expect(names.first()).toHaveText("acme/repo59");
  await expect(page.locator(".rows .chosen")).toHaveCount(1);
});

test("the overlay stays hidden until the backend owns its draft generation", async ({ page }) => {
  await page.goto("/tests/assembly.html");
  await page.waitForFunction(() => window.assembly?.calls().some(({ name }) => name.endsWith(".Begin")));
  expect(await page.evaluate(() => window.assembly.visible())).toBe(false);

  await page.evaluate(() => window.assembly.resolveBegin({ ticket: "", entropy: "4f3a", generation: 7 }));
  await expect.poll(() => page.evaluate(() => window.assembly.visible())).toBe(true);
  await expect.poll(() => page.evaluate(() =>
    window.assembly.calls().some(({ name, args }) => name.endsWith(".Preview") && args[0]?.entropy === "4f3a"),
  )).toBe(true);

  await page.evaluate(() => window.assembly.close());
  await expect.poll(() => page.evaluate(() =>
    window.assembly.calls().some(({ name, args }) => name.endsWith(".End") && args[0] === 7),
  )).toBe(true);
});

test("saved orgs reach the repositories step without it being rebuilt", async ({ page }) => {
  await page.goto("/tests/assembly.html");
  await page.waitForFunction(() => window.assembly?.calls().some(({ name }) => name.endsWith(".Begin")));
  await page.evaluate(() => window.assembly.resolveBegin({ ticket: "", entropy: "4f3a", generation: 7 }));
  await page.getByRole("button", { name: "Choose repositories" }).click();
  await expect(page.locator(".controls .segment")).toHaveText(["acme"]);

  await page.evaluate(() => window.assembly.emit("orgs:changed", ["acme", "other"]));

  await expect(page.locator(".controls .segment")).toHaveText(["acme", "other"]);
  await expect.poll(() => page.evaluate(() =>
    window.assembly.calls().filter(({ name }) => name.endsWith("Repositories.Refresh")).length,
  )).toBe(1);
});

test("the base menu shows the default branch before Repositories.Branches answers, then the full list", async ({ page }) => {
  await page.goto("/tests/assembly.html");
  await page.waitForFunction(() => window.assembly?.calls().some(({ name }) => name.endsWith(".Begin")));
  await page.evaluate(() => window.assembly.resolveBegin({ ticket: "", entropy: "4f3a", generation: 7 }));
  await page.getByRole("button", { name: "Choose repositories" }).click();

  const base = page.locator(".rows button.base");
  await expect(base).toHaveCount(0);
  await page.locator(".rows").getByRole("button", { name: "Editing", exact: true }).click();
  await expect(base).toHaveText("main");
  await base.click();

  const menu = page.locator(".anchor .menu");
  await expect(menu.getByRole("button")).toHaveText(["main", "Listing branches…"]);
  const search = menu.getByRole("textbox", { name: "Search branches" });
  await search.fill("FEAT");
  await expect(menu.getByRole("button")).toHaveText(["Listing branches…"]);
  await expect(menu.getByRole("button", { name: "Listing branches…" })).toBeDisabled();
  await expect.poll(() => page.evaluate(() =>
    window.assembly.calls().some(({ name, args }) => name.endsWith("Repositories.Branches") && args[0] === "acme/api"),
  )).toBe(true);

  await page.evaluate(() =>
    window.assembly.resolveBranches({ branches: ["main", "develop", "feature-x"], default: "main" }),
  );
  await expect(menu.getByRole("button")).toHaveText(["feature-x"]);
  await search.fill("");
  await expect(menu.getByRole("button")).toHaveText(["main", "develop", "feature-x"]);
});

test("a failed branches answer still offers the default branch, disabled with a failure line", async ({ page }) => {
  await page.goto("/tests/assembly.html");
  await page.waitForFunction(() => window.assembly?.calls().some(({ name }) => name.endsWith(".Begin")));
  await page.evaluate(() => window.assembly.resolveBegin({ ticket: "", entropy: "4f3a", generation: 7 }));
  await page.getByRole("button", { name: "Choose repositories" }).click();

  await page.locator(".rows").getByRole("button", { name: "Editing", exact: true }).click();
  await page.locator(".rows button.base").click();
  await page.evaluate(() =>
    window.assembly.resolveBranches({ branches: ["main"], default: "main", error: "listing failed" }),
  );

  const menu = page.locator(".anchor .menu");
  await expect(menu.getByRole("button")).toHaveText(["main", "Couldn't list branches"]);
  await expect(menu.getByRole("button", { name: "Couldn't list branches" })).toBeDisabled();

  await menu.getByRole("textbox", { name: "Search branches" }).fill("missing");
  await expect(menu.getByRole("button")).toHaveText(["Couldn't list branches"]);
  await menu.getByRole("textbox", { name: "Search branches" }).fill("");
  await menu.getByRole("button", { name: "main", exact: true }).click();
  await expect(page.locator(".anchor")).toHaveCount(0);
  await expect(page.locator(".rows button.base")).toHaveText("main");
});

test("a chosen non-default base branch reaches the create payload", async ({ page }) => {
  await page.goto("/tests/assembly.html");
  await page.waitForFunction(() => window.assembly?.calls().some(({ name }) => name.endsWith(".Begin")));
  await page.evaluate(() => window.assembly.resolveBegin({ ticket: "", entropy: "4f3a", generation: 7 }));
  await page.getByRole("button", { name: "Choose repositories" }).click();

  await page.locator(".rows").getByRole("button", { name: "Editing", exact: true }).click();
  await page.locator(".rows button.base").click();
  await page.evaluate(() =>
    window.assembly.resolveBranches({ branches: ["main", "develop"], default: "main" }),
  );
  await page.getByRole("textbox", { name: "Search branches" }).fill("DEVel");
  await page.locator(".anchor .menu").getByRole("button", { name: "develop", exact: true }).click();
  await expect(page.locator(".rows button.base")).toBeFocused();
  await expect(page.locator(".rows button.base")).toHaveText("develop");

  await page.getByRole("button", { name: "Choose an agent" }).click();
  await page.getByRole("button", { name: "Create session" }).click();

  await expect.poll(() => page.evaluate(() =>
    window.assembly.calls().some(
      ({ name, args }) =>
        name.endsWith("Assembly.Create") &&
        args[0]?.repos?.some(
          (repo) => repo.id === "acme/api" && repo.role === "editing" && repo.base === "develop",
        ),
    ),
  )).toBe(true);
});

test("the base button closes the menu it opened", async ({ page }) => {
  await page.goto("/tests/assembly.html");
  await page.waitForFunction(() => window.assembly?.calls().some(({ name }) => name.endsWith(".Begin")));
  await page.evaluate(() => window.assembly.resolveBegin({ ticket: "", entropy: "4f3a", generation: 7 }));
  await page.getByRole("button", { name: "Choose repositories" }).click();
  await page.locator(".rows").getByRole("button", { name: "Editing", exact: true }).click();

  const base = page.locator(".rows button.base");
  await base.click();
  await expect(page.locator(".anchor .menu")).toHaveCount(1);

  await base.click();
  await expect(page.locator(".anchor .menu")).toHaveCount(0);
});

test("a long branch list caps its total popup height and keeps search reachable while scrolling", async ({ page }) => {
  await page.setViewportSize({ width: 1100, height: 650 });
  await page.goto("/tests/assembly.html");
  await page.waitForFunction(() => window.assembly?.calls().some(({ name }) => name.endsWith(".Begin")));
  await page.evaluate(() => window.assembly.resolveBegin({ ticket: "", entropy: "4f3a", generation: 7 }));
  await page.getByRole("button", { name: "Choose repositories" }).click();
  await page.locator(".rows").getByRole("button", { name: "Editing", exact: true }).click();
  await page.locator(".rows button.base").click();

  const menu = page.locator(".anchor .menu");
  const before = await menu.boundingBox();
  await page.evaluate(() =>
    window.assembly.resolveBranches({
      branches: ["main", ...Array.from({ length: 60 }, (_, i) => `topic/${i}`)],
      default: "main",
    }),
  );
  await expect(menu.getByRole("button")).toHaveCount(61);

  const after = await menu.boundingBox();
  expect(after.y).toBe(before.y);
  expect(after.height).toBeLessThanOrEqual(300);
  const room = await page.evaluate(() => window.innerHeight);
  expect(after.y + after.height).toBeLessThanOrEqual(room);
  expect(after.y).toBeGreaterThanOrEqual(0);
  expect(await menu.evaluate((node) => node.scrollHeight > node.clientHeight)).toBe(true);
  const search = menu.getByRole("textbox", { name: "Search branches" });
  await menu.getByRole("button", { name: "topic/59", exact: true }).scrollIntoViewIfNeeded();
  expect(await menu.evaluate((node) => node.scrollTop)).toBeGreaterThan(0);
  const inputBox = await search.boundingBox();
  expect(inputBox.y).toBeGreaterThanOrEqual(after.y);
  expect(inputBox.y + inputBox.height).toBeLessThan(after.y + after.height);
  await search.fill("topic/59");
  await expect(menu.getByRole("button")).toHaveText(["topic/59"]);
  await menu.getByRole("button", { name: "topic/59", exact: true }).click();
  await page.locator(".rows button.base").click();
  await expect(page.getByRole("textbox", { name: "Search branches" })).toHaveValue("");
  await expect(menu.getByRole("button")).toHaveCount(61);
});


async function openBranches(page) {
  await page.goto("/tests/assembly.html");
  await page.waitForFunction(() => window.assembly?.calls().some(({ name }) => name.endsWith(".Begin")));
  await page.evaluate(() => window.assembly.resolveBegin({ ticket: "", entropy: "4f3a", generation: 7 }));
  await page.getByRole("button", { name: "Choose repositories" }).click();
  await page.locator(".rows").getByRole("button", { name: "Editing", exact: true }).click();
  await page.locator(".rows button.base").click();
}

test("branch search ignores case without sorting or changing the base, and clears on reopen", async ({ page }) => {
  await openBranches(page);
  await page.evaluate(() => window.assembly.resolveBranches({
    branches: ["main", "z/Feature", "release", "a/feature", "FEATURE/last"], default: "main",
  }));
  const menu = page.locator(".anchor .menu");
  const search = menu.getByRole("textbox", { name: "Search branches" });
  await search.fill("fEaTuRe");
  await expect(menu.getByRole("button")).toHaveText(["z/Feature", "a/feature", "FEATURE/last"]);
  await expect(page.locator(".rows button.base")).toHaveText("main");
  await search.fill("absent");
  await expect(menu.getByRole("button")).toHaveText(["No matching branches"]);
  await expect(menu.getByRole("button")).toBeDisabled();
  await search.fill("");
  await expect(menu.getByRole("button")).toHaveText(["main", "z/Feature", "release", "a/feature", "FEATURE/last"]);
  await expect(menu.locator("button.active")).toHaveText("main");
  await search.fill("release");
  await page.locator(".rows button.base").click();
  await page.locator(".rows button.base").click();
  await expect(search).toHaveValue("");
  await expect(menu.getByRole("button")).toHaveCount(5);
  expect(await page.evaluate(() => window.assembly.calls().filter(({ name }) => name.endsWith("Repositories.Branches")).length)).toBe(1);
});

test("branch search keeps Enter inside the popup and restores focus after Escape or keyboard selection", async ({ page }) => {
  await openBranches(page);
  await page.evaluate(() => window.assembly.resolveBranches({ branches: ["main", "develop"], default: "main" }));
  const menu = page.locator(".anchor .menu");
  const search = menu.getByRole("textbox", { name: "Search branches" });
  const base = page.locator(".rows button.base");
  await expect(search).toBeFocused();
  await search.press("Enter");
  await expect(page.getByText("Which repositories?", { exact: true })).toBeVisible();
  await expect(search).toBeFocused();
  await search.press("Tab");
  await expect(menu.getByRole("button", { name: "main", exact: true })).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  await expect(search).toBeFocused();
  await search.press("Escape");
  await expect(menu).toHaveCount(0);
  await expect(base).toBeFocused();
  expect(await page.evaluate(() => window.assembly.closeRequests())).toBe(0);
  await base.press("Enter");
  await expect(search).toBeFocused();
  await search.fill("develop");
  await search.press("Tab");
  await page.keyboard.press("Enter");
  await expect(menu).toHaveCount(0);
  await expect(base).toHaveText("develop");
  await expect(base).toBeFocused();
  await base.press("Space");
  await expect(search).toHaveValue("");
  await search.press("Tab");
  await page.keyboard.press("Space");
  await expect(base).toHaveText("main");
  await expect(base).toBeFocused();
});


test("Escape after Tab leaves the branch popup closes it without cancelling assembly", async ({ page }) => {
  await openBranches(page);
  await page.evaluate(() => window.assembly.resolveBranches({ branches: ["main", "develop"], default: "main" }));
  const menu = page.locator(".anchor .menu");
  await expect(menu.getByRole("textbox", { name: "Search branches" })).toBeFocused();
  await page.keyboard.press("Tab");
  await page.keyboard.press("Tab");
  await expect(menu.getByRole("button", { name: "develop", exact: true })).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(page.locator(".rows").getByRole("button", { name: "Off", exact: true })).toBeFocused();
  await expect(menu).toHaveCount(1);
  await page.keyboard.press("Escape");
  await expect(menu).toHaveCount(0);
  await expect(page.locator(".rows button.base")).toBeFocused();
  expect(await page.evaluate(() => window.assembly.closeRequests())).toBe(0);
});

const PROFILED = [
  { id: "claude", label: "Claude Code", profile: "", profileLabel: "", installed: true },
  { id: "claude", label: "Claude Code", profile: "lifesum", profileLabel: "Lifesum", installed: true },
  { id: "codex", label: "Codex", profile: "", profileLabel: "", installed: true },
];
const STATUSES = [
  { runner: "claude", profile: "", state: SIGN_IN_STATES.SIGNED_IN, email: "k@example.com" },
  { runner: "claude", profile: "lifesum", state: SIGN_IN_STATES.SIGNED_OUT },
  { runner: "codex", profile: "", state: SIGN_IN_STATES.SIGNED_IN },
];

const agentStep = async (page, init, arg) => {
  if (init) await page.addInitScript(init, arg);
  await page.goto("/tests/assembly.html");
  await page.waitForFunction(() => window.assembly?.calls().some(({ name }) => name.endsWith(".Begin")));
  await page.evaluate(() => window.assembly.resolveBegin({ ticket: "", entropy: "4f3a", generation: 7 }));
  await page.getByRole("button", { name: "Choose repositories" }).click();
  await page.getByRole("button", { name: "Choose an agent" }).click();
  return page.locator(".group").first().locator(".card");
};

const created = (page) =>
  page.evaluate(() => window.assembly.calls().find(({ name }) => name.endsWith("Assembly.Create"))?.args[0]);

test("a config with no profiles shows today's tiles, with no badge or accounts link", async ({ page }) => {
  const tiles = await agentStep(page);
  await expect(tiles.locator(".title")).toHaveText(["Claude Code", "Codex", "OpenCode", "Antigravity"]);
  await expect(tiles.first()).toHaveClass(/selected/);
  await expect(page.locator(".badge")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Manage accounts" })).toHaveCount(0);
  expect(await page.evaluate((name) => window.assembly.calls().some((call) => call.name === name), PROFILES_STATUS)).toBe(false);

  await page.getByRole("button", { name: "Create session" }).click();
  await expect.poll(() => created(page)).toMatchObject({ runner: "claude", profile: "" });
});

test("each profile is its own tile, badged once the status answers, and a signed-out one is selectable", async ({ page }) => {
  const tiles = await agentStep(page, ([rows, statuses]) => {
    window.runnerRows = rows;
    window.profileStatuses = statuses;
  }, [PROFILED, STATUSES]);
  await expect(tiles.locator(".title")).toHaveText(["Claude Code", "Claude Code · Lifesum", "Codex"]);
  await expect(tiles.nth(0)).toHaveClass(/selected/);
  await expect(tiles.nth(0)).toContainText("Signed in as k@example.com");
  await expect(tiles.nth(1)).toContainText("Signed out");
  await expect(tiles.nth(2).locator(".badge")).toHaveCount(0);
  await expect(tiles.nth(1).locator("img.logo")).toHaveAttribute("src", await tiles.nth(0).locator("img.logo").getAttribute("src"));

  await tiles.nth(1).click();
  await expect(tiles.nth(1)).toHaveClass(/selected/);
  await expect(tiles.nth(0)).not.toHaveClass(/selected/);
  await page.getByRole("button", { name: "Create session" }).click();
  await expect.poll(() => created(page)).toMatchObject({ runner: "claude", profile: "lifesum" });
});

test("Manage accounts opens Settings on the Agents tab, and closing Settings reads the runners again, dropping a deleted choice", async ({ page }) => {
  const tiles = await agentStep(page, ([rows, statuses]) => {
    window.runnerRows = rows;
    window.profileStatuses = statuses;
  }, [PROFILED, STATUSES]);
  await tiles.nth(1).click();
  await page.getByRole("button", { name: "Manage accounts" }).click();
  expect(await page.evaluate(() => window.assembly.settingsRequests())).toEqual(["agents"]);

  await page.evaluate(() => window.assembly.setSettingsOpen(true));
  await page.evaluate((rows) => {
    const personal = { id: "claude", label: "Claude Code", profile: "personal", profileLabel: "Personal", installed: true };
    window.runnerRows = [rows[0], rows[1], personal, rows[2]];
  }, PROFILED);
  await page.evaluate(() => window.assembly.setSettingsOpen(false));

  await expect(tiles.locator(".title")).toHaveText(["Claude Code", "Claude Code · Lifesum", "Claude Code · Personal", "Codex"]);
  await expect(tiles.nth(1)).toHaveClass(/selected/);

  await page.evaluate(() => window.assembly.setSettingsOpen(true));
  await page.evaluate(() => {
    window.runnerRows = window.runnerRows.filter((row) => row.profile !== "lifesum");
  });
  await page.evaluate(() => window.assembly.setSettingsOpen(false));

  await expect(tiles.locator(".title")).toHaveText(["Claude Code", "Claude Code · Personal", "Codex"]);
  await expect(tiles.nth(0)).toHaveClass(/selected/);
  await page.getByRole("button", { name: "Create session" }).click();
  await expect.poll(() => created(page)).toMatchObject({ runner: "claude", profile: "" });
});

// The new-session modal and the add modal share one progress follower; this is
// the new-session half, keyed on the slug the previewed branch names.
test("creating draws progress for the session being created and ignores another's", async ({ page }) => {
  await page.addInitScript(() => (window.previewBranch = "feat/api-cleanup-4f3a"));
  await page.goto("/tests/assembly.html");
  await page.waitForFunction(() => window.assembly?.calls().some(({ name }) => name.endsWith(".Begin")));
  await page.evaluate(() => window.assembly.resolveBegin({ ticket: "", entropy: "4f3a", generation: 7 }));
  await page.getByRole("button", { name: "Choose repositories" }).click();
  await page.getByRole("button", { name: "Choose an agent" }).click();
  await expect(page.locator(".footer .status")).toHaveText("0 repos into api-cleanup-4f3a");
  await page.getByRole("button", { name: "Create session" }).click();
  await expect.poll(() => created(page)).toBeTruthy();

  await page.evaluate((name) => {
    window.assembly.emit(name, { session: "api-cleanup-4f3a", step: "scaffold", status: "completed", percent: 0 });
    window.assembly.emit(name, { session: "squid", step: "scaffold", status: "completed", percent: 0 });
    window.assembly.emit(name, { session: "api-cleanup-4f3a", step: "mirror", status: "started", repo: "acme/api", percent: 0 });
  }, ASSEMBLY_PROGRESS_EVENT);
  await expect(page.locator(".progress .step .label")).toHaveText(["scaffold", "acme/api mirror"]);
});
