import { expect, test } from "@playwright/test";

test("a task with inline code wraps like ordinary prose", async ({ page }) => {
  await page.goto("/tests/markdown.html");
  const layout = await page.evaluate(() => window.taskLayout());

  expect(layout.display).toBe("block");
  expect(layout.task).toBeLessThanOrEqual(layout.paragraph + layout.line);
});


for (const source of ["vault-pane", "ordinary-pane"]) {
  test(`${source} click and context menu preserve fragments within its pane`, async ({ page }) => {
    await page.goto("/tests/markdown.html?links");
    const pane = page.locator(`[data-pane-document="${source}"]`);
    await pane.getByRole("link", {name:"Encoded",exact:true}).click();
    await expect.poll(() => pane.evaluate((node) => node.scrollTop)).toBeGreaterThan(200);
    const other = page.locator(`[data-pane-document="${source === "vault-pane" ? "ordinary-pane" : "vault-pane"}"]`);
    expect(await other.evaluate((node) => node.scrollTop)).toBe(0);
    await pane.evaluate((node) => { node.scrollTop = 0; });
    await pane.getByRole("link", {name:"Duplicate",exact:true}).click({button:"right"});
    await page.getByRole("button", {name:"Open Link",exact:true}).click();
    await expect.poll(() => pane.evaluate((node) => node.scrollTop)).toBeGreaterThan(200);
    const calls = await page.evaluate(() => window.navigationCalls());
    expect(calls).toEqual([{source,href:"target.md#H%C3%A9llo%20%E4%B8%96%E7%95%8C"},{source,href:"target.md#heading-with-spaces-1"}]);
    await pane.evaluate((node) => { node.scrollTop = 0; });
    await pane.getByRole("link", {name:"Raw",exact:true}).click();
    await expect.poll(() => pane.evaluate((node) => node.scrollTop)).toBeLessThan(50);
  });
}

test("legacy badge resets across navigation and revocation and citations stay inert", async ({page})=>{
 await page.goto("/tests/markdown.html?legacy");
 await expect(page.getByRole("note")).toHaveCount(0);
 await page.getByRole("button",{name:"Show legacy"}).click();
 await expect(page.getByRole("note")).toContainText("historical revision unknown");
 await expect(page.getByText("code (github.com/team/repo/main.go#L12; revision unknown)")).toBeVisible();
 await expect(page.getByRole("link")).toHaveCount(0);
 await page.getByRole("button",{name:"Show regular"}).click();
 await expect(page.getByRole("note")).toHaveCount(0);
 await page.getByRole("button",{name:"Show legacy"}).click();
 await page.getByRole("button",{name:"Revoke",exact:true}).click();
 await expect(page.getByRole("note")).toHaveCount(0);
});
