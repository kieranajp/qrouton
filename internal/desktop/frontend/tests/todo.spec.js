import { expect, test } from "@playwright/test";

const open = async (page) => {
  await page.goto("/tests/todo.html");
  await page.waitForSelector("[data-overview]", { state: "attached" });
};

const item = (page, text) => page.locator(`[data-item="${text}"]`);
const sendButton = (page) => page.locator("[data-send]");
const why = (page) => page.locator(".send .why");
const saves = (page) => page.evaluate(() => window.saves());

test("the tally counts open and done items, leaving out the ones not needed", async ({ page }) => {
  await open(page);
  await expect(page.locator("[data-tally]")).toHaveText("1 of 3 done");
  await expect(page.locator('[data-section="Before merge"] [data-section-tally]')).toHaveText("1 of 2 done");
  await expect(item(page, "Roll back the cache")).toHaveAttribute("data-state", "skipped");
  await expect(item(page, "Apply the migration").locator("[data-next]")).toHaveCount(1);
  await expect(item(page, "Apply the migration").locator("[data-blocks]")).toHaveText("Blocks Phase 3");
  await expect(page.locator("[data-blocking]")).toHaveText("1 blocking");
});

test("an item shows its fields, prose and tracked pull request", async ({ page }) => {
  await open(page);
  const apply = item(page, "Apply the migration");
  await expect(apply.locator("[data-where]")).toHaveText("Where enclave · production");
  await expect(apply.locator("[data-run] code")).toHaveText("make migrate ENV=production");
  await expect(apply.locator("[data-check] code")).toHaveText("select count(*) from tenants");
  await expect(apply).toContainText("This can run alongside the items above.");
  await expect(item(page, "Announce in the channel").locator("[data-check]")).toHaveText("the post has a link");
  await expect(item(page, "Announce in the channel").locator("[data-check] code")).toHaveCount(0);
  await expect(item(page, "Merge acme/api#12").locator("[data-pull]")).toContainText("Merged · acme/api#12");
});

test("a tick saves the item's line with its box checked", async ({ page }) => {
  await open(page);
  await item(page, "Apply the migration").getByRole("checkbox").click();
  await expect.poll(async () => (await saves(page)).length).toBe(1);
  const [[id, , text]] = await saves(page);
  expect(id).toBe("w1");
  const todo = await page.evaluate(() => window.TODO);
  expect(text).toBe(todo.replace("- [ ] Apply the migration", "- [x] Apply the migration"));
  await expect(item(page, "Apply the migration").locator("[data-save]")).toHaveText("Saved");
  await expect(page.locator("[data-tally]")).toHaveText("2 of 3 done");
});

test("a note saves after a spaced dash when the field loses focus", async ({ page }) => {
  await open(page);
  const field = item(page, "Announce in the channel").locator("input.note");
  await field.fill("posted at noon");
  expect((await saves(page)).length).toBe(0);
  await page.locator("[data-tally]").click();
  await expect.poll(async () => (await saves(page)).length).toBe(1);
  expect((await saves(page))[0][2]).toContain("- [ ] Announce in the channel — posted at noon\n");
});

test("not needed skips an item, and reopening puts it back in the count", async ({ page }) => {
  await open(page);
  await item(page, "Apply the migration").locator("[data-skip]").click();
  await expect.poll(async () => (await saves(page)).length).toBe(1);
  expect((await saves(page))[0][2]).toContain("- [-] Apply the migration\n");
  await expect(page.locator("[data-tally]")).toHaveText("1 of 2 done");
  await item(page, "Roll back the cache").locator("[data-reopen]").click();
  await expect.poll(async () => (await saves(page)).length).toBe(2);
  expect((await saves(page))[1][2]).toContain("- [ ] Roll back the cache — not needed after all\n");
  await expect(page.locator("[data-tally]")).toHaveText("1 of 3 done");
});

test("a refused save reloads the list and keeps the tick to save again", async ({ page }) => {
  await open(page);
  // The agent adds an item above, so the ticked item no longer sits on the line it was read from.
  const moved = await page.evaluate(() => window.TODO.replace("- [ ] Apply the migration\n", "- [ ] Tag the release\n- [ ] Apply the migration\n"));
  await page.evaluate((text) => window.staleOnce(text), moved);

  const apply = item(page, "Apply the migration");
  await apply.getByRole("checkbox").click();
  await expect(apply.locator(".message")).toContainText("changed on disk");
  await expect(apply.getByRole("checkbox")).toBeChecked();
  await expect(sendButton(page)).toBeDisabled();
  await expect(why(page)).toHaveText("A change did not save. Save it again first.");

  await apply.locator("[data-retry]").click();
  await expect.poll(async () => (await saves(page)).length).toBe(2);
  const text = (await saves(page))[1][2];
  expect(text).toBe(moved.replace("- [ ] Apply the migration\n", "- [x] Apply the migration\n"));
  await expect(item(page, "Tag the release").getByRole("checkbox")).not.toBeChecked();
  await expect(apply.locator("[data-save]")).toHaveText("Saved");
  await expect(sendButton(page)).toBeEnabled();
});

test("a tick whose item text changed meanwhile is detached for the user to redo", async ({ page }) => {
  await open(page);
  const renamed = await page.evaluate(() => window.TODO.replace("Apply the migration", "Apply both migrations"));
  await page.evaluate((text) => window.staleOnce(text), renamed);

  await item(page, "Apply the migration").getByRole("checkbox").click();
  await expect(page.locator('[data-orphan="Apply the migration"]')).toHaveText(/- \[x\] Apply the migration/);
  await expect(item(page, "Apply both migrations").getByRole("checkbox")).not.toBeChecked();
  await expect(sendButton(page)).toBeDisabled();

  await page.locator('[data-orphan="Apply the migration"]').getByRole("button", { name: "Copy" }).click();
  expect(await page.evaluate(() => window.clipboardText)).toBe("- [x] Apply the migration");
  await page.locator('[data-orphan="Apply the migration"]').getByRole("button", { name: "Discard" }).click();
  await expect(page.locator("[data-detached]")).toHaveCount(0);
  await expect(sendButton(page)).toBeEnabled();
});

test("copy writes the item's run command without its backticks", async ({ page }) => {
  await open(page);
  await item(page, "Apply the migration").locator("[data-run]").getByRole("button", { name: "Copy run command" }).click();
  expect(await page.evaluate(() => window.clipboardText)).toBe("make migrate ENV=production");
  await expect(item(page, "Apply the migration").locator("[data-run]").getByRole("button")).toHaveText("Copied");
});

test("tell the agent waits for the save, then makes one bridge call", async ({ page }) => {
  await open(page);
  await page.evaluate(() => window.holdSaves());
  await item(page, "Apply the migration").getByRole("checkbox").click();
  await expect(why(page)).toHaveText("Wait for the save to finish.");
  await expect(sendButton(page)).toBeDisabled();
  await page.evaluate(() => window.releaseSaves());
  await expect(sendButton(page)).toBeEnabled();

  await sendButton(page).click();
  await expect(why(page)).toHaveText("Typed into the conversation.");
  expect(await page.evaluate(() => window.sends())).toEqual([["w1"]]);
});

test("items fit the pane without scrolling sideways", async ({ page }) => {
  await page.setViewportSize({ width: 380, height: 800 });
  await open(page);
  const overflow = await page.locator(".sheet").evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(overflow).toBeLessThanOrEqual(0);
});
