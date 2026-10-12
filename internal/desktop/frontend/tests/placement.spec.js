import { expect, test } from "@playwright/test";

test("rows render ticked beside their routes, with the sessions left alone below", async ({ page }) => {
  await page.goto("/tests/placement.html");
  await expect(page.getByLabel("Kraken")).toBeChecked();
  await expect(page.getByLabel("Squid")).toBeChecked();
  await expect(page.getByText("lifesum to thoughts")).toBeVisible();
  await expect(page.getByTitle("/Users/me/Sync/lifesum").first()).toBeVisible();
  await expect(page.getByText("An agent is running in it")).toBeVisible();
  await expect(page.getByText("usually delete the folder")).toBeVisible();
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

test("only skipped sessions shows their reasons with just a Close button", async ({ page }) => {
  await page.goto("/tests/placement.html?skippedonly");
  await expect(page.getByText("An agent is running in it")).toBeVisible();
  await expect(page.getByText("Every session is where it belongs.")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Move" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Close" })).toHaveCount(1);
  await page.getByRole("button", { name: "Close" }).click();
  await expect.poll(() => page.evaluate(() => window.closes)).toBe(1);
});

test("a pending recovery is listed with its reason and never reads as all clear", async ({ page }) => {
  await page.goto("/tests/placement.html?clear&pending");
  await expect(page.getByText("interrupted move could not be finished: permission denied")).toBeVisible();
  await expect(page.getByText("Every session is where it belongs.")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Move" })).toHaveCount(0);
});

test("a session outside the configured folders names where its thoughts live", async ({ page }) => {
  await page.goto("/tests/placement.html?outside");
  await expect(page.getByText("Its thoughts live in /old/sync/octopus, outside your configured folders. Move them by hand")).toBeVisible();
});
