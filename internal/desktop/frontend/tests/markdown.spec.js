import { expect, test } from "@playwright/test";

test("a task with inline code wraps like ordinary prose", async ({ page }) => {
  await page.goto("/tests/markdown.html");
  const layout = await page.evaluate(() => window.taskLayout());

  expect(layout.display).toBe("block");
  expect(layout.task).toBeLessThanOrEqual(layout.paragraph + layout.line);
});

// An inline checkbox left the space after it on the first line only, so wrapped lines sat one space left of the text.
test("a wrapped task's lines start where its first line's text does", async ({ page }) => {
  await page.goto("/tests/markdown.html");
  const layout = await page.evaluate(() => window.taskLayout());

  expect(layout.lines).toBeGreaterThan(1);
  expect(Math.abs(layout.lastStart - layout.firstStart)).toBeLessThan(1);
});

// Without cloning, a code span split across lines loses its border at the break.
test("inline code keeps its border on every wrapped fragment", async ({ page }) => {
  await page.goto("/tests/markdown.html");
  expect((await page.evaluate(() => window.taskLayout())).codeBreak).toBe("clone");
});
