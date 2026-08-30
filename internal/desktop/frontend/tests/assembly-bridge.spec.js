import { expect, test } from "@playwright/test";

const status = (page) => page.locator(".dialog .status");
const name = (page) => page.locator(".pair input").first();

const opened = async (page, search = "") => {
  await page.goto("/tests/assembly-bridge.html" + search);
  await expect(page.locator(".dialog")).toBeVisible();
};

const toAgentStep = async (page) => {
  await name(page).fill("thing");
  await page.getByRole("button", { name: "Choose repositories →" }).click();
  await page.getByRole("button", { name: "Choose an agent →" }).click();
  await expect(page.getByText("Who runs it, and how?")).toBeVisible();
};

test("a refused agent list says why on the step that would draw it", async ({ page }) => {
  await opened(page, "?fail=Runners");
  await expect(status(page)).toHaveText("no agent command");

  await toAgentStep(page);
  await expect(status(page)).toHaveText("no agent command");
});

test("an empty agent list says why rather than leaving the step blank", async ({ page }) => {
  await opened(page, "?runners=none");
  await toAgentStep(page);
  await expect(status(page)).toContainText("No agent was found on your PATH");
});

test("typing the name asks for one branch preview, and for no rules until advance", async ({ page }) => {
  await opened(page);
  await expect.poll(() => page.evaluate(() => window.bridge.count("Preview"))).toBeGreaterThan(0);
  const settled = await page.evaluate(() => window.bridge.count("Preview"));

  await name(page).pressSequentially("rename the workbench", { delay: 10 });
  await expect
    .poll(() => page.evaluate(() => window.bridge.count("Preview")))
    .toBeGreaterThan(settled);

  expect(await page.evaluate(() => window.bridge.count("Check"))).toBe(0);
  expect(await page.evaluate(() => window.bridge.count("Preview"))).toBeLessThanOrEqual(settled + 3);

  await page.getByRole("button", { name: "Choose repositories →" }).click();
  await expect.poll(() => page.evaluate(() => window.bridge.count("Check"))).toBe(1);
});

test("the description never crosses the bridge, since no rule reads it", async ({ page }) => {
  await opened(page);
  await expect.poll(() => page.evaluate(() => window.bridge.count("Preview"))).toBeGreaterThan(0);
  const asked = await page.evaluate(() => window.bridge.count("Preview"));

  await page.locator("textarea").first().pressSequentially("done looks like this", { delay: 5 });
  await page.waitForTimeout(400);

  expect(await page.evaluate(() => window.bridge.count("Preview"))).toBe(asked);
  expect(await page.evaluate(() => window.bridge.count("Check"))).toBe(0);
});
