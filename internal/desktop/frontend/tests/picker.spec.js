import { expect, test } from "@playwright/test";

const mirror = (session, status, extra = {}) => ({ session, step: "mirror", status, repo: "acme/api", percent: 0, ...extra });

async function add(page) {
  await page.goto("/tests/picker.html");
  await expect(page.locator(".rows .name")).toHaveText(["acme/api"]);
  await page.getByRole("button", { name: "Add repositories" }).click();
  await expect(page.locator(".rows")).toHaveCount(0);
}

// A picker that drops the slug filter would draw another session's assembly.
test("an add draws progress for its own session and ignores another's", async ({ page }) => {
  await add(page);
  await page.evaluate((events) => events.forEach(window.picker.progress), [
    mirror("octopus", "started"),
    mirror("squid", "started"),
    mirror("octopus", "advanced", { phase: "Receiving objects", percent: 40 }),
  ]);

  const steps = page.locator(".progress .step");
  await expect(steps).toHaveCount(2);
  await expect(steps.nth(1).locator(".detail")).toHaveText("Receiving objects");
  await expect(steps.nth(1).locator(".percent")).toHaveText("40%");
});

// A Cancel that is dead while busy leaves a hung clone no way out.
test("Cancel aborts a running add, and its refusal returns to the list with the reason", async ({ page }) => {
  await add(page);
  const cancel = page.getByRole("button", { name: "Cancel" });
  await expect(page.getByRole("button", { name: "Add repositories" })).toBeDisabled();
  await expect(cancel).toBeEnabled();

  await cancel.click();
  await cancel.click();
  await expect.poll(() => page.evaluate(() => window.picker.aborts())).toEqual([["octopus"]]);
  await expect(page.locator(".progress")).toHaveCount(1);

  await page.evaluate(() => window.picker.refuse("context canceled"));
  await expect(page.locator(".rows .name")).toHaveText(["acme/api"]);
  await expect(page.locator(".footer .status")).toHaveText("context canceled");
  expect(await page.evaluate(() => window.picker.closeRequests())).toBe(0);
});

// Escape is Cancel's key, so it must stay live while the add runs too.
test("Escape aborts a running add", async ({ page }) => {
  await add(page);
  await page.keyboard.press("Escape");
  await expect.poll(() => page.evaluate(() => window.picker.aborts())).toEqual([["octopus"]]);
});

// Rows that outlive a refusal would show the last attempt's failure as the retry's.
test("a retry after a refusal starts with no rows", async ({ page }) => {
  await add(page);
  await page.evaluate((event) => window.picker.progress(event), mirror("octopus", "failed", { error: "stalled" }));
  await expect(page.locator(".progress .step")).toHaveCount(1);
  await page.evaluate(() => window.picker.refuse("git stopped producing output"));
  await expect(page.locator(".rows .name")).toHaveText(["acme/api"]);

  await page.getByRole("button", { name: "Add repositories" }).click();
  await expect.poll(() => page.evaluate(() => window.picker.confirms())).toBe(2);
  await expect(page.locator(".progress")).toHaveCount(1);
  await expect(page.locator(".progress .step")).toHaveCount(0);
});
