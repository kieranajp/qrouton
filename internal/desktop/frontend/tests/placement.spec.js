import { expect, test } from "@playwright/test";

test("rows render ticked beside their routes, with the sessions left alone below", async ({ page }) => {
  await page.goto("/tests/placement.html");
  await expect(page.getByLabel("Kraken")).toBeChecked();
  await expect(page.getByLabel("Squid")).toBeChecked();
  await expect(page.getByText("work to default")).toBeVisible();
  await expect(page.getByText("An agent is running in it")).toBeVisible();
  await expect(page.getByText("Sync deletes the folder")).toBeVisible();
});

test("unticking a row leaves it out of Move, and each outcome renders", async ({ page }) => {
  await page.goto("/tests/placement.html");
  await page.getByLabel("Kraken").uncheck();
  await page.getByRole("button", { name: "Move" }).click();
  await expect.poll(() => page.evaluate(() => window.moves)).toEqual([["squid"]]);
  await expect(page.getByText("conflict: already exists")).toBeVisible();
  await expect(page.getByLabel("Kraken")).not.toBeChecked();
});

test("Close calls Close", async ({ page }) => {
  await page.goto("/tests/placement.html");
  await page.getByRole("button", { name: "Close" }).click();
  await expect.poll(() => page.evaluate(() => window.closes)).toBe(1);
});

test("a pending relaunch is mentioned", async ({ page }) => {
  await page.goto("/tests/placement.html?relaunch");
  await expect(page.getByText("waiting for a relaunch")).toBeVisible();
});

test("nothing misplaced shows the all-clear with one Close button", async ({ page }) => {
  await page.goto("/tests/placement.html?clear");
  await expect(page.getByText("Every session is where it belongs.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Move" })).toHaveCount(0);
  await page.getByRole("button", { name: "Close" }).click();
  await expect.poll(() => page.evaluate(() => window.closes)).toBe(1);
});
