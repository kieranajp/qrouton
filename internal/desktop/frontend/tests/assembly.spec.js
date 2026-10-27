import { expect, test } from "@playwright/test";

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
