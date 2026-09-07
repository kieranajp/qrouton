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
  await page.getByRole("button", { name: "Choose repositories →" }).click();
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
  await page.getByRole("button", { name: "Choose repositories →" }).click();

  const base = page.locator(".rows button.base");
  await expect(base).toHaveCount(0);
  await page.locator(".rows").getByRole("button", { name: "Editing", exact: true }).click();
  await expect(base).toHaveText("main ▾");
  await base.click();

  const menu = page.locator(".anchor .menu");
  await expect(menu.getByRole("button")).toHaveText(["main", "Listing branches…"]);
  await expect.poll(() => page.evaluate(() =>
    window.assembly.calls().some(({ name, args }) => name.endsWith("Repositories.Branches") && args[0] === "acme/api"),
  )).toBe(true);

  await page.evaluate(() =>
    window.assembly.resolveBranches({ branches: ["main", "develop", "feature-x"], default: "main" }),
  );
  await expect(menu.getByRole("button")).toHaveText(["main", "develop", "feature-x"]);
});

test("a failed branches answer still offers the default branch, disabled with a failure line", async ({ page }) => {
  await page.goto("/tests/assembly.html");
  await page.waitForFunction(() => window.assembly?.calls().some(({ name }) => name.endsWith(".Begin")));
  await page.evaluate(() => window.assembly.resolveBegin({ ticket: "", entropy: "4f3a", generation: 7 }));
  await page.getByRole("button", { name: "Choose repositories →" }).click();

  await page.locator(".rows").getByRole("button", { name: "Editing", exact: true }).click();
  await page.locator(".rows button.base").click();
  await page.evaluate(() =>
    window.assembly.resolveBranches({ branches: ["main"], default: "main", error: "listing failed" }),
  );

  const menu = page.locator(".anchor .menu");
  await expect(menu.getByRole("button")).toHaveText(["main", "Couldn't list branches"]);
  await expect(menu.getByRole("button", { name: "Couldn't list branches" })).toBeDisabled();

  await menu.getByRole("button", { name: "main", exact: true }).click();
  await expect(page.locator(".anchor")).toHaveCount(0);
  await expect(page.locator(".rows button.base")).toHaveText("main ▾");
});

test("a chosen non-default base branch reaches the create payload", async ({ page }) => {
  await page.goto("/tests/assembly.html");
  await page.waitForFunction(() => window.assembly?.calls().some(({ name }) => name.endsWith(".Begin")));
  await page.evaluate(() => window.assembly.resolveBegin({ ticket: "", entropy: "4f3a", generation: 7 }));
  await page.getByRole("button", { name: "Choose repositories →" }).click();

  await page.locator(".rows").getByRole("button", { name: "Editing", exact: true }).click();
  await page.locator(".rows button.base").click();
  await page.evaluate(() =>
    window.assembly.resolveBranches({ branches: ["main", "develop"], default: "main" }),
  );
  await page.locator(".anchor .menu").getByRole("button", { name: "develop", exact: true }).click();
  await expect(page.locator(".rows button.base")).toHaveText("develop ▾");

  await page.getByRole("button", { name: "Choose an agent →" }).click();
  await page.getByRole("button", { name: "Create session →" }).click();

  await expect.poll(() => page.evaluate(() =>
    window.assembly.calls().some(
      ({ name, args }) =>
        (name.endsWith("Assembly.Create") || name.endsWith("Assembly.Check")) &&
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
  await page.getByRole("button", { name: "Choose repositories →" }).click();
  await page.locator(".rows").getByRole("button", { name: "Editing", exact: true }).click();

  const base = page.locator(".rows button.base");
  await base.click();
  await expect(page.locator(".anchor .menu")).toHaveCount(1);

  await base.click();
  await expect(page.locator(".anchor .menu")).toHaveCount(0);
});

test("a long branch list scrolls inside a menu that does not move when it lands", async ({ page }) => {
  await page.goto("/tests/assembly.html");
  await page.waitForFunction(() => window.assembly?.calls().some(({ name }) => name.endsWith(".Begin")));
  await page.evaluate(() => window.assembly.resolveBegin({ ticket: "", entropy: "4f3a", generation: 7 }));
  await page.getByRole("button", { name: "Choose repositories →" }).click();
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
  const room = await page.evaluate(() => window.innerHeight);
  expect(after.y + after.height).toBeLessThanOrEqual(room);
  expect(await menu.evaluate((node) => node.scrollHeight > node.clientHeight)).toBe(true);
});
