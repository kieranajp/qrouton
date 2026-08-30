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
