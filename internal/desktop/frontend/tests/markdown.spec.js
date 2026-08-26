import { expect, test } from "@playwright/test";

test("a task with inline code wraps like ordinary prose", async ({ page }) => {
  await page.goto("/tests/markdown.html");
  const layout = await page.evaluate(() => window.taskLayout());

  expect(layout.display).toBe("block");
  expect(layout.task).toBeLessThanOrEqual(layout.paragraph + layout.line);
});
